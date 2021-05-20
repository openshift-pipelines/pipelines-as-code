module github.com/openshift-pipelines/pipelines-as-code

go 1.15

require (
	github.com/google/go-cmp v0.5.4
	github.com/google/go-github/v34 v34.0.0
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/jonboulle/clockwork v0.1.1-0.20190114141812-62fb9bc030d1
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/tektoncd/cli v0.17.2
	github.com/tektoncd/hub/api v0.0.0-20210208113044-f2a63f81502c
	github.com/tektoncd/pipeline v0.23.0
	go.uber.org/zap v1.16.0
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	knative.dev/pkg v0.0.0-20210208131226-4b2ae073fa06
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.3
	github.com/kr/pty => github.com/creack/pty v1.1.10
)

// Redirection not working
replace maze.io/x/duration => git.maze.io/go/duration v0.0.0-20160924141736-faac084b6075
