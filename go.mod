module github.com/openshift-pipelines/pipelines-as-code

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/bradleyfalzon/ghinstallation/v2 v2.0.4
	github.com/cloudevents/sdk-go/v2 v2.8.0
	github.com/gfleury/go-bitbucket-v1 v0.0.0-20220301131131-8e7ed04b843e
	github.com/gobwas/glob v0.2.3
	github.com/golang-jwt/jwt/v4 v4.4.0 // indirect
	github.com/google/cel-go v0.10.1
	github.com/google/go-cmp v0.5.7
	github.com/google/go-github/scrape v0.0.0-20220315141941-f85909825349
	github.com/google/go-github/v43 v43.0.0
	github.com/hako/durafmt v0.0.0-20210608085754-5c1018a4e16b
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/jonboulle/clockwork v0.2.2
	github.com/juju/ansiterm v0.0.0-20210929141451-8b71cc96ebdc
	github.com/ktrysmt/go-bitbucket v0.9.40
	github.com/mattn/go-colorable v0.1.12
	github.com/mattn/go-isatty v0.0.14
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/mitchellh/mapstructure v1.4.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.4.0
	github.com/tektoncd/pipeline v0.33.0
	github.com/xanzy/go-gitlab v0.59.0
	github.com/xlzd/gotp v0.0.0-20220110052318-fab697c03c2c // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/sys v0.0.0-20220317061510-51cd9980dadf // indirect
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65 // indirect
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/code-generator v0.22.5
	knative.dev/eventing v0.29.0
	knative.dev/pkg v0.0.0-20220131144930-f4b57aef0006
	sigs.k8s.io/yaml v1.3.0
)
