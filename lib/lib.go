package lib

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetLogs(
	deployment string,
	container string,
	containerSet bool,
	follow bool,
	count int64,
	countSet bool,
) {
	client := getClientSet()
	currentNamespace := getCurrentNamespace()
	pods := getPodList(client, currentNamespace)

	re, err := regexp.Compile(`^` + deployment + `\-[0-9a-f]+\-[0-9a-z]+`)
	if err != nil {
		fmt.Println(err)
	}

	wg := new(sync.WaitGroup)

	for _, pod := range pods.Items {
		if !re.Match([]byte(pod.GetName())) {
			continue
		}
		wg.Add(1)
		go getPodLogs(
			wg,
			*client,
			currentNamespace,
			pod.GetName(),
			container,
			containerSet,
			follow,
			count,
			countSet,
		)
	}

	wg.Wait()
}

func getPodLogs(
	wg *sync.WaitGroup,
	clientSet kubernetes.Clientset,
	namespace string,
	podName string,
	containerName string,
	containerSet bool,
	follow bool,
	count int64,
	countSet bool,
) {
	podLogOptions := getPodLogOptions(follow, count, countSet, containerName, containerSet)
	podLogRequest := clientSet.CoreV1().Pods(namespace).GetLogs(podName, &podLogOptions)
	stream, err := podLogRequest.Stream(context.TODO())

	if err != nil {
		fmt.Println(err)
		wg.Done()
		return
	} else {
		defer stream.Close()
	}

	colorReset := "\033[0m"
	colorBlue := "\033[34m"
	logBuf := ""

	for {
		buf := make([]byte, 2000)
		numBytes, err := stream.Read(buf)

		if err == io.EOF {
			break
		}
		if numBytes == 0 {
			continue
		}
		if err != nil {
			fmt.Println(err)
		}

		message := string(buf[:numBytes])
		for _, c := range message {
			if c == '\n' {
				fmt.Println(colorBlue + podName + " | " + colorReset + logBuf)
				logBuf = ""
			} else {
				logBuf += string(c)
			}
		}
	}

	wg.Done()
}

func getCurrentNamespace() string {
	clientCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		panic(err)
	}
	currentContext := clientCfg.CurrentContext
	currentNamespace := clientCfg.Contexts[currentContext].Namespace
	return currentNamespace
}

func getClientSet() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		panic(err)
	}
	clientSet, _ := kubernetes.NewForConfig(config)

	return clientSet
}

func getPodList(client *kubernetes.Clientset, currentNamespace string) *v1.PodList {
	pods, err := client.CoreV1().Pods(currentNamespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		panic(err)
	}

	return pods
}

func getPodLogOptions(
	follow bool,
	count int64,
	countSet bool,
	containerName string,
	containerSet bool,
) v1.PodLogOptions {
	podLogOptions := v1.PodLogOptions{
		Follow: follow,
	}
	if countSet {
		podLogOptions.TailLines = &count
	}
	if containerSet {
		podLogOptions.Container = containerName
	}

	return podLogOptions
}
