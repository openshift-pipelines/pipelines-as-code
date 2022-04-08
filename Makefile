TARGET_NAMESPACE=pipelines-as-code
QUAY_REPOSITORY=quay.io/openshift-pipeline/pipelines-as-code
QUAY_REPOSITORY_BRANCH=main
GOLANGCI_LINT=golangci-lint
GOFUMPT=gofumpt
LDFLAGS=
OUTPUT_DIR=bin
GO           = go
TIMEOUT_UNIT = 5m


PY_FILES := $(shell find . -type f -regex ".*py" -print)
YAML_FILES := $(shell find . -type f -regex ".*y[a]ml" -print)
MD_FILES := $(shell find . -type f -regex ".*md"  -not -regex '^./vendor/.*'  -not -regex '^./.vale/.*'  -not -regex "^./docs/themes/.*" -not -regex "^./.git/.*" -print)

ifeq ($(PAC_VERSION),)
	PAC_VERSION="$(shell git describe --tags --exact-match 2>/dev/null || echo nightly-`date +'%Y%m%d'`-`git rev-parse --short HEAD`)"
endif
FLAGS += -ldflags "-X github.com/openshift-pipelines/pipelines-as-code/pkg/params/version.Version=$(PAC_VERSION) $(LDFLAGS)"

all: $(OUTPUT_DIR)/pipelines-as-code-controller $(OUTPUT_DIR)/tkn-pac test

FORCE:

.PHONY: vendor
vendor:
	@echo Generating vendor directory
	@go mod tidy && go mod vendor

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

## Tests
TEST_UNIT_TARGETS := test-unit-verbose test-unit-race test-unit-failfast
test-unit-verbose: ARGS=-v
test-unit-failfast: ARGS=-failfast
test-unit-race:    ARGS=-race
$(TEST_UNIT_TARGETS): test-unit
test-clean:  ## Clean testcache
	@echo "Cleaning test cache"
	@go clean -testcache ./...
.PHONY: $(TEST_UNIT_TARGETS) test test-unit
test: test-unit ## Run test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@set -o pipefail ; \
		$(GO) test -timeout $(TIMEOUT_UNIT) $(ARGS) ./... | { grep -v 'no test files'; true; }

.PHONY: test-e2e-cleanup
test-e2e-cleanup: ## cleanup test e2e namespace/pr left open
	@./hack/dev/e2e-tests-cleanup.sh

.PHONY: test-e2e
test-e2e:  test-e2e-cleanup ## run e2e tests
	@go test -failfast -count=1 -tags=e2e $(GO_TEST_FLAGS) ./test

.PHONY: lint
lint: lint-go lint-yaml lint-md ## run all linters

.PHONY: pre-commit
pre-commit: ## Run pre-commit hooks script manually
	@pre-commit run --all-files

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
lint-md: ${MD_FILES} ## runs markdownlint and vale on all markdown files
	@echo "Linting markdown files..."
	@markdownlint $(MD_FILES)
	@echo "Grammar check with vale of documentation..."
	@vale docs/content --minAlertLevel=error --output=line

.PHONY: lint-py
lint-py: ${PY_FILES} ## runs pylint on all python files
	@echo "Linting python files..."
	@pylint $(PY_FILES)

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

check-generated: # vendor update-golden
	@git status -uno |grep -E "modified:[ ]*(vendor/|.*.golden$)" && \
		{ echo "Vendor directory or Golden files has not been generated properly, commit the change first" ; \
		  git status -uno ;	exit 1 ;} || true

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
