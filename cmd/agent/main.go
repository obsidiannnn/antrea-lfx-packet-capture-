package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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

	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	local := 0
	for _, p := range pods.Items {
		if p.Spec.NodeName == nodeName {
			local++
		}
	}

	fmt.Printf("found %d pods on this node\n", local)
}
