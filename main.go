package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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

func canConnect(pod corev1.Pod, timeout time.Duration, port int) bool {
	ip := pod.Status.PodIP

	client := http.Client{Timeout: timeout}
	res, err := client.Get(fmt.Sprintf("http://%s:%d", ip, port))

	if err != nil {
		log.Printf("Error getting %s status: %v", ip, err)
		return false
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
	var timeout = flag.String("t", "10s", "Max time to wait for online pods")

	flag.Parse()

	http.HandleFunc("/", discoveryHandler)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
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

	timeoutDuration, err := time.ParseDuration(*timeout)
	if err != nil {
		log.Fatalf("Unable to parse timeout '%s': %v", *timeout, err)
	}
	timeoutTimer := time.NewTimer(timeoutDuration)
	ticker := time.NewTicker(time.Second)

	for {
		select {

		case <-ticker.C:
			pods := podStatusMap{}
			podList, err := kubeClient.CoreV1().Pods(*namespace).List(opts)
			if err != nil {
				log.Fatalf("Error getting pods: %v", err)
			}

			for _, pod := range podList.Items {
				pods[pod.Name] = canConnect(pod, time.Second, *port)
			}

			if pods.online() >= *waitFor {
				log.Printf("%d pods in inializing state, exiting.", pods.online())
				time.Sleep(time.Second * 5)
				os.Exit(0)
			}

		case <-timeoutTimer.C:
			log.Fatalf("Timed out waiting for online containers")
		}
	}
}
