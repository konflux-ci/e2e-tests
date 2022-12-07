package test

import (
	"fmt"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DecodeTemplate decodes the given string using the given decoder to an instance of a Template
func DecodeTemplate(decoder runtime.Decoder, tmplContent string) (*templatev1.Template, error) {
	tmpl := &templatev1.Template{}
	_, _, err := decoder.Decode([]byte(tmplContent), nil, tmpl)
	if err != nil {
		return nil, err
	}
	return tmpl, err
}

// CreateTemplate joins template skeleton with the given template objects and template params and returns a string containing YAML template
func CreateTemplate(innerObjects TemplateObjects, params TemplateParams) string {
	return fmt.Sprintf(templateSkeleton, innerObjects, params)
}

type TemplateParams string
type TemplateParam string
type TemplateObjects string
type TemplateObject string

// WithParams takes list of template parameters and joins them together
func WithParams(parameters ...TemplateParam) TemplateParams {
	params := ""
	for _, param := range parameters {
		params += string(param)
	}
	return TemplateParams(params)
}

// WithObjects takes list of template objects and joins them together
func WithObjects(innerObjects ...TemplateObject) TemplateObjects {
	objects := ""
	for _, object := range innerObjects {
		objects += string(object)
	}
	return TemplateObjects(objects)
}

const templateSkeleton = `apiVersion: template.openshift.io/v1
kind: Template
metadata:
  labels:
    provider: codeready-toolchain
    version: ${COMMIT}
  name: basic-tier-template
objects:
%s
parameters:
%s
`

// template parameters
const (
	UsernameParam TemplateParam = `
- name: USERNAME
  value: toolchain-dev
  required: true`

	UsernameParamWithoutValue TemplateParam = `
- name: USERNAME
  required: true`

	CommitParam TemplateParam = `
- name: COMMIT
  value: 123abc
  required: true`

	NamespaceParam TemplateParam = `
- name: NAMESPACE
  value: toolchain-host-operator`

	ServSelectorParam TemplateParam = `
- name: SERVICE_SELECTOR
  value: registration-service`
)

// template objects
const (
	Namespace TemplateObject = `
- apiVersion: v1
  kind: Namespace
  metadata:
    annotations:
      openshift.io/description: ${USERNAME}-user
      openshift.io/display-name: ${USERNAME}
      openshift.io/requester: ${USERNAME}
    labels:
      extra: something-extra
      version: ${COMMIT}
    name: ${USERNAME}`

	ServiceAccount TemplateObject = `
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    labels:
      extra: something-extra
    name: registration-service
    namespace: ${NAMESPACE}`

	RoleBinding TemplateObject = `
- apiVersion: authorization.openshift.io/v1
  kind: RoleBinding
  metadata:
    name: ${USERNAME}-edit
    namespace: ${USERNAME}
    labels:
      extra: something-extra
  roleRef:
    kind: ClusterRole
    name: edit
  subjects:
  - kind: User
    name: ${USERNAME}`

	RoleBindingWithExtraUser TemplateObject = `
- apiVersion: authorization.openshift.io/v1
  kind: RoleBinding
  metadata:
    name: ${USERNAME}-edit
    namespace: ${USERNAME}
    labels:
      extra: something-extra
  roleRef:
    kind: ClusterRole
    name: edit
  subjects:
  - kind: User
    name: ${USERNAME}
  - kind: User
    name: extraUser`

	Service TemplateObject = `
- kind: Service
  apiVersion: v1
  metadata:
    name: registration-service
    namespace: ${NAMESPACE}
    labels:
      extra: something-extra
      run: registration-service
  spec:
    selector:
      run: ${SERVICE_SELECTOR}`

	ConfigMap TemplateObject = `
- kind: ConfigMap
  apiVersion: v1
  metadata:
    labels:
      extra: something-extra
    name: registration-service
    namespace: ${NAMESPACE}
  type: Opaque
  data:
    service-selector: ${SERVICE_SELECTOR}`

	NamespaceObj TemplateObject = `{ 
	"apiVersion": "v1",
	"kind": "Namespace",
	"metadata": {
		"annotations": {
			"openshift.io/description": "{{ .Username }}-user",
			"openshift.io/display-name": "{{ .Username }}",
			"openshift.io/requester": "{{ .Username }}"
		},
		"labels": {
			"extra": "something-extra",
			"version": "{{ .Commit }}"
		},
		"name": "{{ .Username }}"
	}
}`

	RolebindingObj TemplateObject = `{
	"apiVersion": "authorization.openshift.io/v1",
	"kind": "RoleBinding",
	"metadata": {
		"name": "{{ .Username }}-edit",
    	"namespace": "{{ .Username }}",
		"labels": {
			"extra": "something-extra"
		}
	},
	"roleRef": {
		"kind": "ClusterRole",
		"name": "edit"
	},
	"subjects": [
		{
			"kind": "User",
			"name": "{{ .Username }}"
		}
	]
}`
)
