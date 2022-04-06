TARGET_NAMESPACE=pipelines-as-code
QUAY_REPOSITORY=quay.io/openshift-pipeline/pipelines-as-code
QUAY_REPOSITORY_BRANCH=main
GO_TEST_FLAGS=-v -cover
GOLANGCI_LINT=golangci-lint
GOFUMPT=gofumpt
LDFLAGS=
OUTPUT_DIR=bin

YAML_FILES := $(shell find . -type f -regex ".*y[a]ml" -print)
MD_FILES := $(shell find . -type f -regex ".*md"  -not -regex '^./vendor/.*' -not -regex "^./docs/themes/.*" -not -regex "^./.git/.*" -print)

ifeq ($(PAC_VERSION),)
	PAC_VERSION="$(shell git describe --tags --exact-match 2>/dev/null || echo nightly-`date +'%Y%m%d'`-`git rev-parse --short HEAD`)"
endif
FLAGS += -ldflags "-X github.com/openshift-pipelines/pipelines-as-code/pkg/params/version.Version=$(PAC_VERSION) $(LDFLAGS)"

all: $(OUTPUT_DIR)/pipelines-as-code-controller $(OUTPUT_DIR)/tkn-pac test

FORCE:

.PHONY: vendor
vendor:
	go mod tidy && go mod vendor

$(OUTPUT_DIR)/%: cmd/% FORCE
	go build -mod=vendor $(FLAGS)  -v -o $@ ./$<

.PHONY: releaseyaml
releaseyaml: ## Generate release.yaml, use it like this `make releaseyaml|kubectl apply -f-`
	@env TARGET_REPO=$(QUAY_REPOSITORY) TARGET_BRANCH=$(QUAY_REPOSITORY_BRANCH) TARGET_NAMESPACE=$(TARGET_NAMESPACE) \
		PAC_VERSION=$(PAC_VERSION) \
		./hack/generate-releaseyaml.sh

.PHONY: releaseko
releaseko: ## Generate release.yaml with ko but changing the target_namespace and branch if needed
	@env TARGET_BRANCH=$(QUAY_REPOSITORY_BRANCH) TARGET_NAMESPACE=$(TARGET_NAMESPACE) \
		PAC_VERSION=$(PAC_VERSION) \
		./hack/generate-releaseyaml.sh ko

check: lint test

.PHONY: test
test: test-unit ## run all tests

.PHONY: test-e2e-cleanup
test-e2e-cleanup: ## cleanup test e2e namespace/pr left open
	@./hack/dev/e2e-tests-cleanup.sh

.PHONY: test-e2e
test-e2e:  test-e2e-cleanup ## run e2e tests
	@go test -failfast -count=1 -tags=e2e $(GO_TEST_FLAGS) ./test

.PHONY: lint
lint: lint-go lint-yaml lint-md ## run all linters

.PHONY: lint-go
lint-go: ## runs go linter on all go files
	@echo "Linting go files..."
	@$(GOLANGCI_LINT) run ./... --modules-download-mode=vendor \
							--max-issues-per-linter=0 \
							--max-same-issues=0 \
							--deadline 5m

.PHONY: lint-yaml
lint-yaml: ${YAML_FILES} ## runs yamllint on all yaml files
	@echo "Linting yaml files..."
	@yamllint -c .yamllint $(YAML_FILES)


.PHONY: lint-md
lint-md: ${MD_FILES} ## runs markdownlint on all markdown files
	@echo "Linting markdown files..."
	@markdownlint $(MD_FILES)

.PHONY: test-unit
test-unit: ## run unit tests
	@echo "Cleaning test cache"
	@go clean -testcache ./...
	@echo "Running unit tests..."
	@go test -failfast $(GO_TEST_FLAGS) ./...

.PHONY: update-golden
update-golden: ./vendor ## run unit tests (updating golden files)
	@echo "Running unit tests to update golden files..."
	@./hack/update-golden.sh

.PHONY: generated
generated: update-golden fumpt ## generate all files that needs to be generated

.PHONY: html-coverage
html-coverage: ./vendor ## generate html coverage
	@mkdir -p tmp
	@go test -coverprofile=tmp/c.out ./.../ && go tool cover -html=tmp/c.out

.PHONY: clean
clean: ## clean build artifacts
	rm -fR bin

.PHONY: fmt ## formats the GO code(excludes vendors dir)
fmt:
	@go fmt `go list ./... | grep -v /vendor/`

.PHONY: fumpt ## formats the GO code with gofumpt(excludes vendors dir)
fumpt:
	@$(GOFUMPT) -w test/*/*/*go test/*go

.PHONY: help
help: ## print this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {gsub("\\\\n",sprintf("\n%22c",""), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
