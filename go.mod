module github.com/redhat-appstudio/e2e-tests

go 1.19

require (
	github.com/avast/retry-go/v4 v4.3.3
	github.com/codeready-toolchain/api v0.0.0-20230502074949-f8c1d80bac96
	github.com/codeready-toolchain/toolchain-e2e v0.0.0-20220525131508-60876bfb99d3
	github.com/devfile/library/v2 v2.2.1-0.20230418160146-e75481b7eebd
	github.com/enterprise-contract/enterprise-contract-controller/api v0.0.0-20230327185456-5befd172d558
	github.com/go-git/go-git/v5 v5.6.1
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572
	github.com/gofri/go-github-ratelimit v1.0.3-0.20230428184158-a500e14de53f
	github.com/google/go-containerregistry v0.15.2
	github.com/google/go-github/v44 v44.1.0
	github.com/google/uuid v1.3.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/gosuri/uitable v0.0.4
	github.com/h2non/gock v1.2.0
	github.com/magefile/mage v1.13.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.10
	github.com/openshift/api v0.0.0-20230213134911-7ba313770556
	github.com/openshift/client-go v0.0.0-20221019143426-16aed247da5c
	github.com/openshift/library-go v0.0.0-20220525173854-9b950a41acdc
	github.com/openshift/oc v0.0.0-alpha.0.0.20220614012638-35c7eeb5274e
	github.com/redhat-appstudio/application-api v0.0.0-20230704143842-035c661f115f
	github.com/redhat-appstudio/build-service v0.0.0-20230727072250-19c7a8b5c63d
	github.com/redhat-appstudio/image-controller v0.0.0-20230619094421-a6d78d062948
	github.com/redhat-appstudio/integration-service v0.0.0-20230427084439-306b4611389b
	github.com/redhat-appstudio/jvm-build-service v0.0.0-20230718114256-6342e1e18ee8
	github.com/redhat-appstudio/managed-gitops/backend v0.0.0-20220506042230-3a79f373a001
	github.com/redhat-appstudio/managed-gitops/backend-shared v0.0.0
	github.com/redhat-appstudio/release-service v0.0.0-20230530123949-005cd468d6a7
	github.com/redhat-appstudio/remote-secret v0.0.0-20230713072146-a6094c712436
	github.com/redhat-appstudio/service-provider-integration-operator v0.2023.22-0.20230713080056-eae17aa8c172
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.8.4
	github.com/tektoncd/cli v0.29.1
	github.com/tektoncd/pipeline v0.45.0
	golang.org/x/oauth2 v0.10.0
	golang.org/x/tools v0.9.3
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.27.0
	k8s.io/apimachinery v0.27.3
	k8s.io/cli-runtime v0.25.4
	k8s.io/client-go v1.5.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.100.1
	k8s.io/kubectl v0.24.1
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2
	knative.dev/pkg v0.0.0-20230125083639-408ad0773f47
	kubevirt.io/qe-tools v0.1.8
	sigs.k8s.io/controller-runtime v0.14.6
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/antlr/antlr4 => github.com/antlr/antlr4 v0.0.0-20211106181442-e4c1a74c66bd
	github.com/apcera/gssapi => github.com/openshift/gssapi v0.0.0-20161010215902-5fb4217df13b
	github.com/redhat-appstudio/managed-gitops/backend-shared => github.com/redhat-appstudio/managed-gitops/backend-shared v0.0.0-20220506042230-3a79f373a001
	k8s.io/api => k8s.io/api v0.25.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.25.5
	k8s.io/client-go => k8s.io/client-go v0.25.5
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.13.0
)

require (
	code.gitea.io/sdk/gitea v0.15.1 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200907061046-05415f1de66d // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230106234847-43070de90fa1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230417170513-8ee5748c52b5 // indirect
	github.com/acomagu/bufpipe v1.0.4 // indirect
	github.com/aws/aws-sdk-go v1.44.191 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/bluekeyes/go-gitdiff v0.7.0 // indirect
	github.com/bradleyfalzon/ghinstallation/v2 v2.1.1-0.20221216144751-8f41e6541ca6 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v0.0.0-20160711120539-c6fed771bfd5 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/containerd/containerd v1.7.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/containers/image/v5 v5.15.0 // indirect
	github.com/containers/libtrust v0.0.0-20190913040956-14b96171aa3b // indirect
	github.com/containers/ocicrypt v1.1.6 // indirect
	github.com/containers/storage v1.33.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/devfile/api/v2 v2.2.1-alpha.0.20230413012049-a6c32fca0dbd // indirect
	github.com/devfile/registry-support/index/generator v0.0.0-20230123181701-4de4dadb13e7 // indirect
	github.com/devfile/registry-support/registry-library v0.0.0-20230123181701-4de4dadb13e7 // indirect
	github.com/docker/cli v23.0.5+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker v23.0.5+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-errors/errors v1.4.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.4.1 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-github/v45 v45.2.0 // indirect
	github.com/google/go-github/v48 v48.2.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20230406165453-00490a63f317 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.0 // indirect
	github.com/h2non/parth v0.0.0-20190131123155-b4df798d6542 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jenkins-x/go-scm v1.13.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/buildkit v0.11.4 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc3 // indirect
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.1 // indirect
	github.com/openshift-pipelines/pipelines-as-code v0.17.3 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/prometheus/statsd_exporter v0.23.0 // indirect
	github.com/redhat-appstudio/application-service v0.0.0-20230717184417-67d31a01a776 // indirect
	github.com/redhat-developer/alizer/go v0.0.0-20230516215932-135a2bb3fb90 // indirect
	github.com/redhat-developer/gitops-generator v0.0.0-20230614175323-aff86c6bc55e // indirect
	github.com/rivo/uniseg v0.4.2 // indirect
	github.com/russross/blackfriday v1.6.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/shurcooL/githubv4 v0.0.0-20221229060216-a8d4a561cc93 // indirect
	github.com/shurcooL/graphql v0.0.0-20220606043923-3cf50f8a0a29 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/pflag v1.0.6-0.20210604193023-d5e0c0615ace // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/xanzy/go-gitlab v0.83.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.11.0 // indirect
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.12.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/term v0.10.0 // indirect
	golang.org/x/text v0.11.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/api v0.126.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230530153820-e85fd2cbaebc // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230530153820-e85fd2cbaebc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230530153820-e85fd2cbaebc // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.27.0 // indirect
	k8s.io/component-base v0.27.0 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/pod-security-admission v0.26.1 // indirect
	oras.land/oras-go v1.1.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
