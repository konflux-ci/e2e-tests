module github.com/redhat-appstudio/e2e-tests

go 1.16

require (
	github.com/argoproj/argo-cd/v2 v2.1.7
	github.com/argoproj/gitops-engine v0.4.1
	github.com/tektoncd/pipeline v0.32.1
	github.com/onsi/ginkgo/v2 v2.0.0-rc2
	github.com/onsi/gomega v1.17.0
	github.com/redhat-appstudio/application-service v0.0.0-20220106201253-98d082511fd2
	github.com/tektoncd/pipeline v0.30.0
	golang.org/x/net v0.0.0-20211123203042-d83791d6bcd9 // indirect
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v11.0.1-0.20190816222228-6d55c1b1f1ca+incompatible
	sigs.k8s.io/controller-runtime v0.10.3
)
