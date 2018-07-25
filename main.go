package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type podStatusMap map[string]bool

func (pods podStatusMap) online() int {
	var count int
	for _, podOnline := range pods {
		if podOnline {
			count++
		}
	}
	return count
}

func canConnect(pod *corev1.Pod, timeout time.Duration, port *int) bool {
	ip := pod.Status.PodIP

	client := http.Client{Timeout: timeout}
	res, err := client.Get(fmt.Sprintf("http://%s:%d", ip, port))

	if err != nil {
		log.Printf("Error getting %s status: %v", ip, err)
	}

	if res.StatusCode == http.StatusOK {
		return true
	}

	return false
}

func discoveryHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func main() {
	var namespace = flag.String("n", "default", "Kubernetes Namespace")
	var port = flag.Int("p", 10100, "Discovery Server Port")
	var labelSelector = flag.String("l", "", "Label Selector")
	var waitFor = flag.Int("w", 3, "Wait for this many pods")

	http.HandleFunc("/", discoveryHandler)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			log.Fatalf("Unable to start http discovery server: %v", err)
		}
	}()

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
		pods[pod.Name] = false
	}

	opts.Watch = true
	opts.ResourceVersion = podList.ResourceVersion
	podWatch, err := kubeClient.CoreV1().Pods(*namespace).Watch(opts)
	if err != nil {
		log.Fatalf("Error watching pods: %v", err)
	}

	for pods.online() < *waitFor {
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
				pods[pod.Name] = canConnect(pod, time.Second, port)
			case "Deleted":
				delete(pods, pod.Name)
			}
		}
	}

	log.Printf("%d pods in inializing state, exiting.", pods.online())
}
