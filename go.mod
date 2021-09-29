module github.com/openshift-pipelines/pipelines-as-code

go 1.15

require (
	github.com/AlecAivazis/survey/v2 v2.2.12
	github.com/briandowns/spinner v1.16.0
	github.com/davecgh/go-spew v1.1.1
	github.com/gobwas/glob v0.2.3
	github.com/google/go-cmp v0.5.6
	github.com/google/go-github/v35 v35.3.0
	github.com/hako/durafmt v0.0.0-20210601083242-f49dacec7612
	github.com/jonboulle/clockwork v0.1.1-0.20190114141812-62fb9bc030d1
	github.com/mattn/go-colorable v0.1.2
	github.com/mattn/go-isatty v0.0.13
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/tektoncd/pipeline v0.24.3
	go.uber.org/zap v1.16.0
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	knative.dev/pkg v0.0.0-20210331065221-952fdd90dbb0
	sigs.k8s.io/yaml v1.2.0
)
