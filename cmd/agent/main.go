package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const annotationKey = "tcpdump.antrea.io"

var (
	activePod string
	activeCmd *exec.Cmd
	mu        sync.Mutex
)

func main() {
	fmt.Println("antrea capture agent starting")

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		panic("NODE_NAME env var is required")
	}
	fmt.Printf("running on node: %s\n", nodeName)

	// --- Kubernetes config ---
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

	// 🔑 declare first
	var stopCaptureLocked func()

	startCapture := func(pod *v1.Pod, n int) {
		key := pod.Namespace + "/" + pod.Name

		mu.Lock()
		defer mu.Unlock()

		// Stop any existing capture
		if activePod != "" && activePod != key {
			stopCaptureLocked()
		}

		// Already capturing this pod
		if activePod == key {
			return
		}

		file := fmt.Sprintf("/tmp/capture-%s.pcap", pod.Name)

		cmd := exec.Command(
			"tcpdump",
			"-i", "eth0",
			"-C", "1",
			"-W", strconv.Itoa(n),
			"-w", file,
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Printf("failed to start tcpdump for %s: %v\n", key, err)
			return
		}

		activeCmd = cmd
		activePod = key
		fmt.Printf("[START] tcpdump for pod %s (N=%d)\n", key, n)
	}

	stopCapture := func(pod *v1.Pod) {
		key := pod.Namespace + "/" + pod.Name

		mu.Lock()
		defer mu.Unlock()

		if activePod == key {
			stopCaptureLocked()
		}
	}

	// 🔑 define after declaration
	stopCaptureLocked = func() {
		if activeCmd == nil {
			return
		}

		fmt.Printf("[STOP] tcpdump for pod %s\n", activePod)

		_ = activeCmd.Process.Kill()
		_ = activeCmd.Wait()

		_ = exec.Command("sh", "-c", "rm -f /tmp/capture-*.pcap*").Run()

		activeCmd = nil
		activePod = ""
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

			val, has := pod.Annotations[annotationKey]
			if has {
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					startCapture(pod, n)
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

	select {}
}
