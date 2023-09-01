package tekton

import (
	"context"
	"os/exec"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create a tekton task and return the task or error.
func (t *TektonController) CreateTask(task *v1beta1.Task, ns string) (*v1beta1.Task, error) {
	return t.PipelineClient().TektonV1beta1().Tasks(ns).Create(context.TODO(), task, metav1.CreateOptions{})
}

// CreateSkopeoCopyTask creates a skopeo copy task in the given namespace.
func (t *TektonController) CreateSkopeoCopyTask(namespace string) error {
	_, err := exec.Command(
		"oc",
		"apply",
		"-f",
		"https://api.hub.tekton.dev/v1/resource/tekton/task/skopeo-copy/0.2/raw",
		"-n",
		namespace).Output()

	return err
}

// GetTask returns the requested Task object.
func (t *TektonController) GetTask(name, namespace string) (*v1beta1.Task, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	task := v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := t.KubeRest().Get(context.TODO(), namespacedName, &task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// DeleteAllTasksInASpecificNamespace removes all Tasks from a given repository. Useful when creating a lot of resources and wanting to remove all of them.
func (t *TektonController) DeleteAllTasksInASpecificNamespace(namespace string) error {
	return t.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.Task{}, crclient.InNamespace(namespace))
}
