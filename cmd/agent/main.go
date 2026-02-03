package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const annotationKey = "tcpdump.antrea.io"

func main() {
	fmt.Println("antrea capture agent starting")

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		panic("NODE_NAME env var is required")
	}
	fmt.Printf("running on node: %s\n", nodeName)

	// --- Kubernetes config (IN-CLUSTER first, then KUBECONFIG, then HOME) ---
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Track active tcpdump processes per Pod UID
	active := make(map[string]*exec.Cmd)

	startCapture := func(pod *v1.Pod, n int) {
		uid := string(pod.UID)
		file := fmt.Sprintf("/tmp/capture-%s.pcap", pod.Name)

		cmd := exec.Command(
			"tcpdump",
			"-C", "1",
			"-W", strconv.Itoa(n),
			"-w", file,
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Printf("failed to start tcpdump for %s/%s: %v\n", pod.Namespace, pod.Name, err)
			return
		}

		active[uid] = cmd
		fmt.Printf("[START] tcpdump for pod %s/%s (N=%d)\n", pod.Namespace, pod.Name, n)
	}

	stopCapture := func(pod *v1.Pod) {
		uid := string(pod.UID)

		if cmd, ok := active[uid]; ok {
			_ = cmd.Process.Kill()
			delete(active, uid)
		}

		files, _ := filepath.Glob("/tmp/capture-*")
		for _, f := range files {
			_ = os.Remove(f)
		}

		fmt.Printf("[STOP] tcpdump for pod %s/%s\n", pod.Namespace, pod.Name)
	}

	factory := informers.NewSharedInformerFactory(clientset, time.Minute)
	podInformer := factory.Core().V1().Pods().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if pod.Spec.NodeName != nodeName {
				return
			}
			if val, ok := pod.Annotations[annotationKey]; ok {
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					startCapture(pod, n)
				}
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			if pod.Spec.NodeName != nodeName {
				return
			}
			if val, ok := pod.Annotations[annotationKey]; ok {
				if _, exists := active[string(pod.UID)]; !exists {
					if n, err := strconv.Atoi(val); err == nil && n > 0 {
						startCapture(pod, n)
					}
				}
			} else {
				stopCapture(pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			stopCapture(obj.(*v1.Pod))
		},
	})

	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	// Block forever
	select {}
}
