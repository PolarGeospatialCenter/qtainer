package main

import (
	"flag"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type podStatusMap map[string]corev1.PodStatus

func (pods podStatusMap) initializing() int {
	var count int
	for _, pod := range pods {
		for _, icStatus := range pod.InitContainerStatuses {
			if icStatus.State.Running != nil {
				count++
				break
			}
		}
	}
	return count
}

func main() {
	var namespace = flag.String("n", "default", "Kubernetes Namespace")
	var labelSelector = flag.String("l", "", "Label Selector")
	var waitFor = flag.Int("w", 3, "Wait for this many pods")

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Unable to get incluster config: %v", err)
	}

	kubeClient := kubernetes.NewForConfigOrDie(cfg)

	opts := metav1.ListOptions{}
	opts.IncludeUninitialized = true

	if *labelSelector != "" {
		opts.LabelSelector = *labelSelector
	}

	pods := podStatusMap{}

	podList, err := kubeClient.CoreV1().Pods(*namespace).List(opts)
	if err != nil {
		log.Fatalf("Error getting pods: %v", err)
	}

	for _, pod := range podList.Items {
		pods[pod.Name] = pod.Status
	}

	opts.Watch = true
	opts.ResourceVersion = podList.ResourceVersion
	podWatch, err := kubeClient.CoreV1().Pods(*namespace).Watch(opts)
	if err != nil {
		log.Fatalf("Error watching pods: %v", err)
	}

	for pods.initializing() < *waitFor {
		log.Printf("Waiting for pod updates...")
		select {
		case event := <-podWatch.ResultChan():
			if event.Type == "Error" {
				log.Fatalf("Got error from watch: %v", event.Object)
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Printf("Got object of unexpected type from watch %T", event.Object)
				continue
			}

			switch event.Type {
			case "Added":
				fallthrough
			case "Modified":
				pods[pod.Name] = pod.Status
			case "Deleted":
				delete(pods, pod.Name)
			}
		}
	}

	log.Printf("%d pods in inializing state, exiting.", pods.initializing())
}
