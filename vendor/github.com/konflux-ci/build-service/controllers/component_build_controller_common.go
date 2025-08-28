/*
Copyright 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/pkg/boerrors"
	. "github.com/konflux-ci/build-service/pkg/common"
	l "github.com/konflux-ci/build-service/pkg/logs"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type BuildPipeline struct {
	Name   string `json:"name,omitempty"`
	Bundle string `json:"bundle,omitempty"`
}

type pipelineConfig struct {
	DefaultPipelineName string          `yaml:"default-pipeline-name"`
	Pipelines           []BuildPipeline `yaml:"pipelines"`
}

// getGitProvider returns git provider name based on the repository url, e.g. github, gitlab, etc or git-privider annotation
func getGitProvider(component appstudiov1alpha1.Component) (string, error) {
	allowedGitProviders := map[string]bool{"github": true, "gitlab": true, "bitbucket": true}
	gitProvider := ""

	if component.Spec.Source.GitSource == nil {
		err := fmt.Errorf("git source URL is not set for %s Component in %s namespace", component.Name, component.Namespace)
		return "", err
	}
	sourceUrl := component.Spec.Source.GitSource.URL

	if strings.HasPrefix(sourceUrl, "git@") {
		// git@github.com:redhat-appstudio/application-service.git
		sourceUrl = strings.TrimPrefix(sourceUrl, "git@")
		host := strings.Split(sourceUrl, ":")[0]
		gitProvider = strings.Split(host, ".")[0]
	} else {
		// https://github.com/redhat-appstudio/application-service
		u, err := url.Parse(sourceUrl)
		if err != nil {
			return "", err
		}
		uParts := strings.Split(u.Hostname(), ".")
		if len(uParts) == 1 {
			gitProvider = uParts[0]
		} else {
			gitProvider = uParts[len(uParts)-2]
		}
	}

	var err error
	if !allowedGitProviders[gitProvider] {
		// Self-hosted git provider, check for git-provider annotation on the component
		gitProviderAnnotationValue := component.GetAnnotations()[GitProviderAnnotationName]
		if gitProviderAnnotationValue != "" {
			if allowedGitProviders[gitProviderAnnotationValue] {
				gitProvider = gitProviderAnnotationValue
			} else {
				err = fmt.Errorf("unsupported \"%s\" annotation value: %s", GitProviderAnnotationName, gitProviderAnnotationValue)
			}
		} else {
			err = fmt.Errorf("self-hosted git provider is not specified via \"%s\" annotation in the component", GitProviderAnnotationName)
		}
	}

	return gitProvider, err
}

// SetDefaultBuildPipelineComponentAnnotation sets default build pipeline to component pipeline annotation
func (r *ComponentBuildReconciler) SetDefaultBuildPipelineComponentAnnotation(ctx context.Context, component *appstudiov1alpha1.Component) error {
	log := ctrllog.FromContext(ctx)
	pipelinesConfigMap := &corev1.ConfigMap{}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: buildPipelineConfigMapResourceName, Namespace: BuildServiceNamespaceName}, pipelinesConfigMap); err != nil {
		if errors.IsNotFound(err) {
			return boerrors.NewBuildOpError(boerrors.EBuildPipelineConfigNotDefined, err)
		}
		return err
	}

	buildPipelineData := &pipelineConfig{}
	if err := yaml.Unmarshal([]byte(pipelinesConfigMap.Data[buildPipelineConfigName]), buildPipelineData); err != nil {
		return boerrors.NewBuildOpError(boerrors.EBuildPipelineConfigNotValid, err)
	}

	pipelineAnnotation := fmt.Sprintf("{\"name\":\"%s\",\"bundle\":\"%s\"}", buildPipelineData.DefaultPipelineName, "latest")
	if component.Annotations == nil {
		component.Annotations = make(map[string]string)
	}
	component.Annotations[defaultBuildPipelineAnnotation] = pipelineAnnotation

	if err := r.Client.Update(ctx, component); err != nil {
		log.Error(err, fmt.Sprintf("failed to update component with default pipeline annotation %s", defaultBuildPipelineAnnotation))
		return err
	}
	log.Info(fmt.Sprintf("updated component with default pipeline annotation %s", defaultBuildPipelineAnnotation))
	return nil
}

// GetBuildPipelineFromComponentAnnotation parses pipeline annotation on component and returns build pipeline
func (r *ComponentBuildReconciler) GetBuildPipelineFromComponentAnnotation(ctx context.Context, component *appstudiov1alpha1.Component) (*tektonapi.PipelineRef, error) {
	buildPipeline, err := readBuildPipelineAnnotation(component)
	if err != nil {
		return nil, err
	}
	if buildPipeline == nil {
		err := fmt.Errorf("missing or empty pipeline annotation: %s, will add default one to the component", component.Annotations[defaultBuildPipelineAnnotation])
		return nil, boerrors.NewBuildOpError(boerrors.EMissingPipelineAnnotation, err)
	}
	if buildPipeline.Bundle == "" || buildPipeline.Name == "" {
		err = fmt.Errorf("missing name or bundle in pipeline annotation: name=%s bundle=%s", buildPipeline.Name, buildPipeline.Bundle)
		return nil, boerrors.NewBuildOpError(boerrors.EWrongPipelineAnnotation, err)
	}
	finalBundle := buildPipeline.Bundle

	if buildPipeline.Bundle == "latest" {
		pipelinesConfigMap := &corev1.ConfigMap{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: buildPipelineConfigMapResourceName, Namespace: BuildServiceNamespaceName}, pipelinesConfigMap); err != nil {
			if errors.IsNotFound(err) {
				return nil, boerrors.NewBuildOpError(boerrors.EBuildPipelineConfigNotDefined, err)
			}
			return nil, err
		}

		buildPipelineData := &pipelineConfig{}
		if err := yaml.Unmarshal([]byte(pipelinesConfigMap.Data[buildPipelineConfigName]), buildPipelineData); err != nil {
			return nil, boerrors.NewBuildOpError(boerrors.EBuildPipelineConfigNotValid, err)
		}

		for _, pipeline := range buildPipelineData.Pipelines {
			if pipeline.Name == buildPipeline.Name {
				finalBundle = pipeline.Bundle
				break
			}
		}

		// requested pipeline was not found in configMap
		if finalBundle == "latest" {
			err = fmt.Errorf("invalid pipeline name in pipeline annotation: name=%s", buildPipeline.Name)
			return nil, boerrors.NewBuildOpError(boerrors.EBuildPipelineInvalid, err)
		}
	}

	pipelineRef := &tektonapi.PipelineRef{
		ResolverRef: tektonapi.ResolverRef{
			Resolver: "bundles",
			Params: []tektonapi.Param{
				{Name: "name", Value: *tektonapi.NewStructuredValues(buildPipeline.Name)},
				{Name: "bundle", Value: *tektonapi.NewStructuredValues(finalBundle)},
				{Name: "kind", Value: *tektonapi.NewStructuredValues("pipeline")},
			},
		},
	}
	return pipelineRef, nil
}

func (r *ComponentBuildReconciler) ensurePipelineServiceAccount(ctx context.Context, namespace string) (*corev1.ServiceAccount, error) {
	log := ctrllog.FromContext(ctx)

	pipelinesServiceAccount := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: buildPipelineServiceAccountName, Namespace: namespace}, pipelinesServiceAccount)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("Failed to read service account %s in namespace %s", buildPipelineServiceAccountName, namespace), l.Action, l.ActionView)
			return nil, err
		}
		// Create service account for the build pipeline
		buildPipelineSA := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      buildPipelineServiceAccountName,
				Namespace: namespace,
			},
		}
		if err := r.Client.Create(ctx, &buildPipelineSA); err != nil {
			log.Error(err, fmt.Sprintf("Failed to create service account %s in namespace %s", buildPipelineServiceAccountName, namespace), l.Action, l.ActionAdd)
			return nil, err
		}
		return r.ensurePipelineServiceAccount(ctx, namespace)
	}
	return pipelinesServiceAccount, nil
}

func (r *ComponentBuildReconciler) linkSecretToServiceAccount(ctx context.Context, secretName, serviceAccountName, namespace string, isPullSecret bool) (bool, error) {
	log := ctrllog.FromContext(ctx)

	if secretName == "" {
		// The secret is empty, no updates needed
		return false, nil
	}

	serviceAccount := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: namespace}, serviceAccount)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		log.Error(err, fmt.Sprintf("Failed to read service account %s in namespace %s", serviceAccountName, namespace), l.Action, l.ActionView)
		return false, err
	}

	isNewSecretLinked := false

	shouldAddLink := true
	for _, secret := range serviceAccount.Secrets {
		if secret.Name == secretName {
			// The secret is present in the service account, no updates needed
			shouldAddLink = false
			break
		}
	}
	if shouldAddLink {
		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: secretName, Namespace: namespace})
	}
	isNewSecretLinked = shouldAddLink

	if isPullSecret {
		shouldAddLink = true
		for _, secret := range serviceAccount.ImagePullSecrets {
			if secret.Name == secretName {
				// The secret is present in the service account, no updates needed
				shouldAddLink = false
				break
			}
		}
		if shouldAddLink {
			serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
		}
		isNewSecretLinked = isNewSecretLinked || shouldAddLink
	}

	// Update service account if needed
	if isNewSecretLinked {
		err := r.Client.Update(ctx, serviceAccount)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update service account %s", serviceAccount.Name), l.Action, l.ActionUpdate)
			return false, err
		}
		log.Info(fmt.Sprintf("Service Account %s updated with secret %s", serviceAccount.Name, secretName), l.Action, l.ActionUpdate)
		return true, nil
	}
	return false, nil
}

// unlinkSecretFromServiceAccount ensures that the given secret is not linked with the provided service account.
// Returns true if the secret was unlinked, false if the link didn't exist.
func (r *ComponentBuildReconciler) unlinkSecretFromServiceAccount(ctx context.Context, secretNameToRemove, serviceAccountName, namespace string) (bool, error) {
	log := ctrllog.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: namespace}, serviceAccount)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		log.Error(err, fmt.Sprintf("Failed to read service account %s in namespace %s", serviceAccountName, namespace), l.Action, l.ActionView)
		return false, err
	}

	isSecretUnlinked := false
	// Remove secret from secrets list
	for index, credentialSecret := range serviceAccount.Secrets {
		if credentialSecret.Name == secretNameToRemove {
			secrets := make([]corev1.ObjectReference, 0, len(serviceAccount.Secrets)-1)
			if len(serviceAccount.Secrets) != 1 {
				secrets = append(secrets, serviceAccount.Secrets[:index]...)
				secrets = append(secrets, serviceAccount.Secrets[index+1:]...)
			}
			serviceAccount.Secrets = secrets
			isSecretUnlinked = true
			break
		}
	}
	// Remove secret from pull secrets list
	for index, pullSecret := range serviceAccount.ImagePullSecrets {
		if pullSecret.Name == secretNameToRemove {
			secrets := make([]corev1.LocalObjectReference, 0, len(serviceAccount.ImagePullSecrets)-1)
			if len(serviceAccount.ImagePullSecrets) != 1 {
				secrets = append(secrets, serviceAccount.ImagePullSecrets[:index]...)
				secrets = append(secrets, serviceAccount.ImagePullSecrets[index+1:]...)
			}
			serviceAccount.ImagePullSecrets = secrets
			isSecretUnlinked = true
			break
		}
	}

	if isSecretUnlinked {
		if err := r.Client.Update(ctx, serviceAccount); err != nil {
			log.Error(err, fmt.Sprintf("Unable to update pipeline service account %v", serviceAccount), l.Action, l.ActionUpdate)
			return false, err
		}
		log.Info(fmt.Sprintf("Removed %s secret link from %s service account", secretNameToRemove, serviceAccount.Name), l.Action, l.ActionUpdate)
	}
	return isSecretUnlinked, nil
}

func getContainerImageRepositoryForComponent(component *appstudiov1alpha1.Component) string {
	if component.Spec.ContainerImage != "" {
		return getContainerImageRepository(component.Spec.ContainerImage)
	}
	imageRepo, _, err := getComponentImageRepoAndSecretNameFromImageAnnotation(component)
	if err == nil && imageRepo != "" {
		return imageRepo
	}
	return ""
}

// getContainerImageRepository removes tag or SHA has from container image reference
func getContainerImageRepository(image string) string {
	if strings.Contains(image, "@") {
		// registry.io/user/image@sha256:586ab...d59a
		return strings.Split(image, "@")[0]
	}
	// registry.io/user/image:tag
	return strings.Split(image, ":")[0]
}

// getComponentImageRepoAndSecretNameFromImageAnnotation parses image.redhat.com/image annotation
// for image repository and secret name to access it.
// If image.redhat.com/image is not set, the procedure returns empty values.
func getComponentImageRepoAndSecretNameFromImageAnnotation(component *appstudiov1alpha1.Component) (string, string, error) {
	type RepositoryInfo struct {
		Image  string `json:"image"`
		Secret string `json:"secret"`
	}

	var repoInfo RepositoryInfo
	if imageRepoDataJson, exists := component.Annotations[ImageRepoAnnotationName]; exists {
		if err := json.Unmarshal([]byte(imageRepoDataJson), &repoInfo); err != nil {
			return "", "", boerrors.NewBuildOpError(boerrors.EFailedToParseImageAnnotation, err)
		}
		return repoInfo.Image, repoInfo.Secret, nil
	}
	return "", "", nil
}

// mergeAndSortTektonParams merges additional params into existing params by adding new or replacing existing values.
func mergeAndSortTektonParams(existedParams, additionalParams []tektonapi.Param) []tektonapi.Param {
	var params []tektonapi.Param
	paramsMap := make(map[string]tektonapi.Param)
	for _, p := range existedParams {
		paramsMap[p.Name] = p
	}
	for _, p := range additionalParams {
		paramsMap[p.Name] = p
	}
	for _, v := range paramsMap {
		params = append(params, v)
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
	return params
}

func generateVolumeClaimTemplate() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("1Gi"),
				},
			},
		},
	}
}

func getPathContext(gitContext, dockerfileContext string) string {
	if gitContext == "" && dockerfileContext == "" {
		return ""
	}
	separator := string(filepath.Separator)
	path := filepath.Join(gitContext, dockerfileContext)
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, separator)
	return path
}

func getPipelineNameAndBundle(pipelineRef *tektonapi.PipelineRef) (string, string, error) {
	if pipelineRef.Resolver != "" && pipelineRef.Resolver != "bundles" {
		return "", "", boerrors.NewBuildOpError(
			boerrors.EUnsupportedPipelineRef,
			fmt.Errorf("unsupported Tekton resolver %q", pipelineRef.Resolver),
		)
	}

	name := pipelineRef.Name
	var bundle string

	for _, param := range pipelineRef.Params {
		switch param.Name {
		case "name":
			name = param.Value.StringVal
		case "bundle":
			bundle = param.Value.StringVal
		}
	}

	if name == "" || bundle == "" {
		return "", "", boerrors.NewBuildOpError(
			boerrors.EMissingParamsForBundleResolver,
			fmt.Errorf("missing name or bundle in pipelineRef: name=%s bundle=%s", name, bundle),
		)
	}

	return name, bundle, nil
}

func readBuildPipelineAnnotation(component *appstudiov1alpha1.Component) (*BuildPipeline, error) {
	if component.Annotations == nil {
		return nil, nil
	}

	requestedPipeline, requestedPipelineExists := component.Annotations[defaultBuildPipelineAnnotation]
	if requestedPipelineExists && requestedPipeline != "" {
		buildPipeline := &BuildPipeline{}
		buildPipelineBytes := []byte(requestedPipeline)

		if err := json.Unmarshal(buildPipelineBytes, buildPipeline); err != nil {
			return nil, boerrors.NewBuildOpError(boerrors.EFailedToParsePipelineAnnotation, err)
		}
		return buildPipeline, nil
	}
	return nil, nil
}
