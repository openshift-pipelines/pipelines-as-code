module github.com/openshift-pipelines/pipelines-as-code

go 1.15

require (
	github.com/google/go-cmp v0.5.4
	github.com/google/go-github/v34 v34.0.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/tektoncd/cli v0.17.2
	github.com/tektoncd/pipeline v0.23.0
	go.uber.org/zap v1.16.0
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/code-generator v0.21.0
	knative.dev/pkg v0.0.0-20210208131226-4b2ae073fa06
)

replace (
	// Needed until kustomize is updated in the k8s repos:
	// https://github.com/kubernetes-sigs/kustomize/issues/1500
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.3
	github.com/kr/pty => github.com/creack/pty v1.1.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.7
)
