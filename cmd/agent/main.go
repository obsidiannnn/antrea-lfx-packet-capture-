package main

import (
	"fmt"
	"os"
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

	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	active := make(map[string]int)

	factory := informers.NewSharedInformerFactory(clientset, time.Minute)
	podInformer := factory.Core().V1().Pods().Informer()

	handle := func(pod *v1.Pod) {
		if pod.Spec.NodeName != nodeName {
			return
		}

		uid := string(pod.UID)
		val, ok := pod.Annotations[annotationKey]

		if ok {
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return
			}

			prev, exists := active[uid]
			if !exists {
				active[uid] = n
				fmt.Printf("[START] capture pod %s/%s (N=%d)\n", pod.Namespace, pod.Name, n)
			} else if prev != n {
				active[uid] = n
				fmt.Printf("[RESTART] capture pod %s/%s (N=%d)\n", pod.Namespace, pod.Name, n)
			}
		} else {
			if _, exists := active[uid]; exists {
				delete(active, uid)
				fmt.Printf("[STOP] capture pod %s/%s\n", pod.Namespace, pod.Name)
			}
		}
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handle(obj.(*v1.Pod))
		},
		UpdateFunc: func(_, newObj interface{}) {
			handle(newObj.(*v1.Pod))
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if _, exists := active[string(pod.UID)]; exists {
				delete(active, string(pod.UID))
				fmt.Printf("[STOP] capture pod %s/%s (deleted)\n", pod.Namespace, pod.Name)
			}
		},
	})

	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	select {}
}
