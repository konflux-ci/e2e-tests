package framework

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"regexp"
	"strings"
	"time"
)

func ReportFailure(f **Framework) func() {
	namespaces := map[string]string{
		"Build Service":       "build-service",
		"JVM Build Service":   "jvm-build-service",
		"Application Service": "application-service",
		"Image Controller":    "image-controller"}
	return func() {
		report := CurrentSpecReport()
		if report.Failed() {
			now := time.Now()
			AddReportEntry("timing", "Test started at "+report.StartTime.String()+
				"\nTest ended at "+now.String())
			fwk := *f
			if fwk == nil {
				return
			}
			for k, v := range namespaces {
				msg := "\n========= " + k + " =========\n\n"
				podInterface := fwk.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(v)
				pods, err := podInterface.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					msg += "Error listing pods: " + err.Error() + "\n"
				} else {
					for _, pod := range pods.Items {
						containers := []corev1.Container{}
						containers = append(containers, pod.Spec.InitContainers...)
						containers = append(containers, pod.Spec.Containers...)
						for _, container := range containers {
							req := podInterface.GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
							logs, err := innerDumpPod(req, container.Name)
							if err != nil {
								msg += "Error getting logs: " + err.Error() + "\n"
							} else {
								msg += FilterLogs(logs, report.StartTime) + "\n"
							}
						}
					}
				}
				AddReportEntry(v, msg)
			}
		}
	}
}

func FilterLogs(logs string, start time.Time) string {

	//bit of a hack, the logs are in different formats and are not always valid JSON
	//just look for RFC 3339 dates line by line, once we find one after the start time dump the
	//rest of the lines
	lines := strings.Split(logs, "\n")
	ret := []string{}
	rfc3339Pattern := `(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2}))`

	re := regexp.MustCompile(rfc3339Pattern)
	for pos, i := range lines {
		match := re.FindStringSubmatch(i)

		if match != nil {
			dateString := match[1]
			ts, err := time.Parse(time.RFC3339, dateString)
			if err != nil {
				ret = append(ret, "Invalid Time, unable to parse date: "+i)
			} else if ts.Equal(start) || ts.After(start) {
				ret = append(ret, lines[pos:]...)
				break
			}
		}
	}

	return strings.Join(ret, "\n")

}

func innerDumpPod(req *rest.Request, containerName string) (string, error) {
	var readCloser io.ReadCloser
	var err error
	readCloser, err = req.Stream(context.TODO())
	if err != nil {
		print(fmt.Sprintf("error getting pod logs for container %s: %s", containerName, err.Error()))
		return "", err
	}
	defer func(readCloser io.ReadCloser) {
		err := readCloser.Close()
		if err != nil {
			print(fmt.Sprintf("Failed to close ReadCloser reading pod logs for container %s: %s", containerName, err.Error()))
		}
	}(readCloser)
	var b []byte
	b, err = io.ReadAll(readCloser)
	return string(b), err
}
