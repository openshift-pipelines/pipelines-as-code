TARGET_NAMESPACE=pipelines-as-code
GOLANGCI_LINT=golangci-lint
GOFUMPT=gofumpt
TKN_BINARY_NAME := tkn
LDFLAGS=
OUTPUT_DIR=bin
GO           = go
TIMEOUT_UNIT = 20m
TIMEOUT_E2E  = 20m
GO_TEST_FLAGS +=
SHELL := /bin/bash


PY_FILES := $(shell find . -type f -regex ".*py" -print)
YAML_FILES := $(shell find . -type f -regex ".*y[a]ml" -print)
MD_FILES := $(shell find . -type f -regex ".*md"  -not -regex '^./vendor/.*'  -not -regex '^./.vale/.*'  -not -regex "^./docs/themes/.*" -not -regex "^./.git/.*" -print)

ifeq ($(PAC_VERSION),)
	PAC_VERSION="$(shell git describe --tags --exact-match 2>/dev/null || echo nightly-`date +'%Y%m%d'`-`git rev-parse --short HEAD`)"
endif
FLAGS += -ldflags "-X github.com/openshift-pipelines/pipelines-as-code/pkg/params/version.Version=$(PAC_VERSION) $(LDFLAGS) -X github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings.TknBinaryName=$(TKN_BINARY_NAME)"


all: $(OUTPUT_DIR)/pipelines-as-code-controller $(OUTPUT_DIR)/tkn-pac test

FORCE:

.PHONY: vendor
vendor:
	@echo Generating vendor directory
	@go mod tidy -compat=1.17 && go mod vendor

$(OUTPUT_DIR)/%: cmd/% FORCE
	go build -mod=vendor $(FLAGS)  -v -o $@ ./$<


$(OUTPUT_DIR)/%.exe: cmd/% FORCE
	env GOOS=windows GOARCH=amd64 go build -mod=vendor $(FLAGS)  -v -o $@ ./$<

check: lint test

allinone: ## Run kind all in one install
	@cd ./hack/dev/kind/;./install.sh

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
test: test-clean test-unit ## Run test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@set -o pipefail ; \
		$(GO) test $(GO_TEST_FLAGS) -timeout $(TIMEOUT_UNIT) $(ARGS) ./... | { grep -v 'no test files'; true; }

.PHONY: test-e2e-cleanup
test-e2e-cleanup: ## cleanup test e2e namespace/pr left open
	@./hack/dev/e2e-tests-cleanup.sh

.PHONY: test-e2e
test-e2e:  test-e2e-cleanup ## run e2e tests
	@go test $(GO_TEST_FLAGS) -timeout $(TIMEOUT_E2E)  -failfast -count=1 -tags=e2e $(GO_TEST_FLAGS) ./test

.PHONY: lint
lint: lint-go lint-yaml lint-md lint-py ## run all linters

.PHONY: pre-commit
pre-commit: ## Run pre-commit hooks script manually
	@pre-commit run --all-files

.PHONY: lint-go
lint-go: ## runs go linter on all go files
	@echo "Linting go files..."
	@$(GOLANGCI_LINT) run ./... --modules-download-mode=vendor \
							--max-issues-per-linter=0 \
							--max-same-issues=0 \
							--deadline $(TIMEOUT_UNIT)

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

.PHONY: docs-dev
docs-dev: ## preview live your docs with hugo
	@hugo server -s docs/

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
	@find test pkg -name '*.go'|xargs -P4 $(GOFUMPT) -w -extra

.PHONY: help
help: ## print this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {gsub("\\\\n",sprintf("\n%22c",""), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
