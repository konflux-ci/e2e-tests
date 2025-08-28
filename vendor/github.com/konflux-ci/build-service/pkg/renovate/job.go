package renovate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/konflux-ci/build-service/pkg/common"
	"github.com/konflux-ci/build-service/pkg/logs"
)

const (
	TasksPerJob                = 20
	InstallationsPerJobEnvName = "RENOVATE_INSTALLATIONS_PER_JOB"
	TimeToLiveOfJob            = 24 * time.Hour
	RenovateImageEnvName       = "RENOVATE_IMAGE"
	DefaultRenovateImageUrl    = "quay.io/redhat-appstudio/renovate:v37.74.1"
)

// JobCoordinator is responsible for creating and managing renovate k8s jobs
type JobCoordinator struct {
	tasksPerJob      int
	renovateImageUrl string
	debug            bool
	client           client.Client
	scheme           *runtime.Scheme
}

func NewJobCoordinator(client client.Client, scheme *runtime.Scheme) *JobCoordinator {
	var tasksPerJobInt int
	tasksPerJobStr := os.Getenv(InstallationsPerJobEnvName)
	if regexp.MustCompile(`^\d{1,2}$`).MatchString(tasksPerJobStr) {
		tasksPerJobInt, _ = strconv.Atoi(tasksPerJobStr)
		if tasksPerJobInt == 0 {
			tasksPerJobInt = TasksPerJob
		}
	} else {
		tasksPerJobInt = TasksPerJob
	}
	renovateImageUrl := os.Getenv(RenovateImageEnvName)
	if renovateImageUrl == "" {
		renovateImageUrl = DefaultRenovateImageUrl
	}
	return &JobCoordinator{tasksPerJob: tasksPerJobInt, renovateImageUrl: renovateImageUrl, client: client, scheme: scheme, debug: false}
}

func (j *JobCoordinator) Execute(ctx context.Context, tasks []*Task) error {

	if len(tasks) == 0 {
		return nil
	}
	log := logger.FromContext(ctx)

	timestamp := time.Now().Unix()
	name := fmt.Sprintf("renovate-job-%d-%s", timestamp, RandomString(5))
	log.Info(fmt.Sprintf("Creating renovate job %s for %d unique sets of scm repositories", name, len(tasks)))

	secretTokens := map[string]string{}
	configMapData := map[string]string{}
	var renovateCmd []string
	for _, task := range tasks {
		taskId := RandomString(5)
		secretTokens[taskId] = task.Token

		config, err := json.Marshal(task.JobConfig())
		if err != nil {
			return err
		}
		configMapData[fmt.Sprintf("%s.json", taskId)] = string(config)

		log.Info(fmt.Sprintf("Creating renovate config map entry with length %d and value %s", len(config), config))
		renovateCmd = append(renovateCmd,
			fmt.Sprintf("RENOVATE_TOKEN=$TOKEN_%s RENOVATE_CONFIG_FILE=/configs/%s.json renovate || true", taskId, taskId),
		)
	}
	if len(renovateCmd) == 0 {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: BuildServiceNamespaceName,
		},
		StringData: secretTokens,
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: BuildServiceNamespaceName,
		},
		Data: configMapData,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: BuildServiceNamespaceName,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(1)),
			TTLSecondsAfterFinished: ptr.To(int32(TimeToLiveOfJob.Seconds())),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: name,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: name},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "renovate",
							Image: j.renovateImageUrl,
							EnvFrom: []corev1.EnvFromSource{
								{
									Prefix: "TOKEN_",
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: name,
										},
									},
								},
							},
							Command: []string{"bash", "-c", strings.Join(renovateCmd, "; ")},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      name,
									MountPath: "/configs",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	if j.debug {
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "LOG_LEVEL", Value: "debug"})
	}
	if err := j.client.Create(ctx, secret); err != nil {
		return err
	}
	if err := j.client.Create(ctx, configMap); err != nil {
		return err
	}
	if err := j.client.Create(ctx, job); err != nil {
		return err
	}
	log.Info("renovate job created", "jobname", job.Name, "tasks", len(tasks), logs.Action, logs.ActionAdd)
	if err := controllerutil.SetOwnerReference(job, secret, j.scheme); err != nil {
		return err
	}
	if err := j.client.Update(ctx, secret); err != nil {
		return err
	}

	if err := controllerutil.SetOwnerReference(job, configMap, j.scheme); err != nil {
		return err
	}
	if err := j.client.Update(ctx, configMap); err != nil {
		return err
	}
	return nil
}

func (j *JobCoordinator) ExecuteWithLimits(ctx context.Context, tasks []*Task) error {

	for i := 0; i < len(tasks); i += j.tasksPerJob {
		end := i + j.tasksPerJob

		if end > len(tasks) {
			end = len(tasks)
		}
		err := j.Execute(ctx, tasks[i:end])
		if err != nil {
			return err
		}
	}
	return nil

}
