module github.com/redhat-appstudio/e2e-tests

go 1.16

require (
	github.com/averageflow/gohooks/v2 v2.1.0
	github.com/devfile/library v1.2.0
	github.com/google/uuid v1.3.0
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.19.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/redhat-appstudio/application-service v0.0.0-20220312031926-2976522a9052
	github.com/redhat-appstudio/managed-gitops/backend v0.0.0-20220506042230-3a79f373a001
	github.com/stretchr/testify v1.7.1
	github.com/tektoncd/pipeline v0.32.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.10.3
)

replace github.com/redhat-appstudio/managed-gitops/backend-shared => github.com/redhat-appstudio/managed-gitops/backend-shared v0.0.0-20220506042230-3a79f373a001
