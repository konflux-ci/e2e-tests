module github.com/redhat-appstudio/e2e-tests

go 1.16

require (
	github.com/averageflow/gohooks/v2 v2.1.0
	github.com/google/uuid v1.3.0
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.19.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/redhat-appstudio/application-service v0.0.0-20220509130137-3bdbc6eecda4
	github.com/redhat-appstudio/service-provider-integration-operator v0.5.0
	github.com/stretchr/testify v1.7.1
	github.com/tektoncd/pipeline v0.33.0
	google.golang.org/api v0.74.0 // indirect
	google.golang.org/genproto v0.0.0-20220413183235-5e96e2839df9 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.40.1
	sigs.k8s.io/controller-runtime v0.11.0
)
