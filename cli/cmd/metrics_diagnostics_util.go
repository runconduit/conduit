package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/linkerd/linkerd2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
)

// shared between metrics and diagnostics command
type metricsResult struct {
	pod       string
	container string
	metrics   []byte
	err       error
}
type byResult []metricsResult

func (s byResult) Len() int {
	return len(s)
}
func (s byResult) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byResult) Less(i, j int) bool {
	return s[i].pod < s[j].pod || ((s[i].pod == s[j].pod) && s[i].container < s[j].container)
}

// getResponse makes a http Get request to the passed url and returns the response/error
func getResponse(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// getContainerMetrics returns the metrics exposed by a container on the passed in portName
func getContainerMetrics(
	k8sAPI *k8s.KubernetesAPI,
	pod corev1.Pod,
	container corev1.Container,
	emitLogs bool,
	portName string,
) ([]byte, error) {
	portForward, err := k8s.NewContainerMetricsForward(k8sAPI, pod, container, emitLogs, portName)
	if err != nil {
		return nil, err
	}

	defer portForward.Stop()
	if err = portForward.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running port-forward: %s", err)
	}

	metricsURL := portForward.URLFor("/metrics")
	bytes, err := getResponse(metricsURL)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// getAllContainersWithPort returns all the containers within
// a pod which exposes metrics at a port with name portName
func getAllContainersWithPort(
	pod corev1.Pod,
	portName string,
) ([]corev1.Container, error) {
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod not running: %s", pod.GetName())
	}
	var containers []corev1.Container

	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == portName {
				containers = append(containers, c)
			}
		}
	}
	return containers, nil
}

// getMetrics returns the metrics exposed by all the containers of the passed in list of pods
// which exposes their metrics at portName
func getMetrics(
	k8sAPI *k8s.KubernetesAPI,
	pods []corev1.Pod,
	portName string,
	waitingTime time.Duration,
	emitLogs bool,
) []metricsResult {
	var results []metricsResult

	resultChan := make(chan metricsResult)
	var activeRoutines int32
	for _, pod := range pods {
		containers, err := getAllContainersWithPort(pod, portName)
		if err != nil {
			fmt.Println(err)
			continue
		}

		for i := range containers {
			atomic.AddInt32(&activeRoutines, 1)
			go func(c corev1.Container) {
				bytes, err := getContainerMetrics(k8sAPI, pod, c, emitLogs, portName)

				resultChan <- metricsResult{
					pod:       pod.GetName(),
					container: c.Name,
					metrics:   bytes,
					err:       err,
				}

				atomic.AddInt32(&activeRoutines, -1)
			}(containers[i])
		}
	}

	for {
		select {
		case result := <-resultChan:
			results = append(results, result)
		case <-time.After(waitingTime):
			break // timed out
		}
		if atomic.LoadInt32(&activeRoutines) == 0 {
			break
		}
	}

	sort.Sort(byResult(results))

	return results
}
