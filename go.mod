module github.com/openshift-pipelines/pipelines-as-code

go 1.19

require (
	code.gitea.io/sdk/gitea v0.15.1
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/bradleyfalzon/ghinstallation/v2 v2.6.0
	github.com/cloudevents/sdk-go/v2 v2.14.0
	github.com/fvbommel/sortorder v1.1.0
	github.com/gfleury/go-bitbucket-v1 v0.0.0-20230626192437-8d7be5866751
	github.com/gobwas/glob v0.2.3
	github.com/google/cel-go v0.17.1
	github.com/google/go-cmp v0.5.9
	github.com/google/go-github/scrape v0.0.0-20230803055657-a8da03b06966
	github.com/google/go-github/v53 v53.2.0
	github.com/hako/durafmt v0.0.0-20210608085754-5c1018a4e16b
	github.com/jonboulle/clockwork v0.4.0
	github.com/juju/ansiterm v1.0.0
	github.com/ktrysmt/go-bitbucket v0.9.65
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.19
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/mitchellh/mapstructure v1.5.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.8.4
	github.com/tektoncd/pipeline v0.50.0
	github.com/xanzy/go-gitlab v0.90.0
	go.opencensus.io v0.24.0
	go.uber.org/zap v1.25.0
	golang.org/x/exp v0.0.0-20230801115018-d63ba01acd4b
	golang.org/x/oauth2 v0.11.0
	golang.org/x/sync v0.3.0
	golang.org/x/text v0.13.0
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/v3 v3.5.0
	k8s.io/api v0.27.4
	k8s.io/apimachinery v0.27.4
	k8s.io/client-go v1.5.2
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	knative.dev/eventing v0.38.0
	knative.dev/pkg v0.0.0-20230718152110-aef227e72ead
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230717121422-5aa5874ade95 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230321174746-8dcc6526cfb1 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/go-fed/httpsig v1.1.0 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/pprof v0.0.0-20230111200839-76d1ae5aea2b // indirect
	github.com/onsi/ginkgo/v2 v2.9.7 // indirect
	github.com/onsi/gomega v1.27.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230803162519-f966b187b2e5 // indirect
	k8s.io/kube-openapi v0.0.0-20230718181711-3c0fae5ee9fd // indirect
)

replace (
	code.gitea.io/sdk/gitea => code.gitea.io/sdk/gitea v0.15.1-0.20230509035020-970776d1c1e9
	k8s.io/api => k8s.io/api v0.25.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.5
	k8s.io/client-go => k8s.io/client-go v0.25.9
	k8s.io/code-generator => k8s.io/code-generator v0.25.9
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20230515203736-54b630e78af5
	knative.dev/pkg => knative.dev/pkg v0.0.0-20230418073056-dfad48eaa5d0
)

require (
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200907061046-05415f1de66d // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	contrib.go.opencensus.io/exporter/zipkin v0.1.2 // indirect
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudevents/sdk-go/observability/opencensus/v2 v2.14.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-containerregistry v0.16.1 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.4 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/golang-lru v0.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/prometheus/statsd_exporter v0.24.0 // indirect
	github.com/rickb777/date v1.20.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/xlzd/gotp v0.1.0 // indirect
	go.uber.org/automaxprocs v1.5.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/api v0.134.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/grpc v1.57.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apiextensions-apiserver v0.27.4 // indirect
	k8s.io/klog/v2 v2.100.1
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
)
