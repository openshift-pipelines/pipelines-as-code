YAML_FILES := $(shell find . -type f -regex ".*y[a]ml" -print)

ifneq ($(FLAGS),)
	LDFLAGS := -ldflags "$(FLAGS)"
endif

all: bin/tkn test

FORCE:

vendor:
	@go mod vendor

bin/%: cmd/% FORCE
	go build -mod=vendor $(LDFLAGS) -v -o $@ ./$<

check: lint test

.PHONY: test
test: test-unit ## run all tests

.PHONY: lint
lint: lint-go lint-yaml ## run all linters

.PHONY: lint-go
lint-go: ## runs go linter on all go files
	@echo "Linting go files..."
	@golangci-lint run ./... --modules-download-mode=vendor \
							--max-issues-per-linter=0 \
							--max-same-issues=0 \
							--deadline 5m

.PHONY: lint-yaml
lint-yaml: ${YAML_FILES} ## runs yamllint on all yaml files
	@echo "Linting yaml files..."
	@yamllint -c .yamllint $(YAML_FILES)

.PHONY: test-unit
test-unit: ./vendor ## run unit tests
	@echo "Running unit tests..."
	@go test -failfast -v -cover ./...

.PHONY: html-coverage
html-coverage: ./vendor ## generate html coverage
	@mkdir -p tmp
	@go test -coverprofile=tmp/c.out ./.../ && go tool cover -html=tmp/c.out

.PHONY: clean
clean: ## clean build artifacts
	rm -fR bin VERSION

.PHONY: fmt ## formats the GO code(excludes vendors dir)
fmt:
	@go fmt `go list ./... | grep -v /vendor/`

.PHONY: help
help: ## print this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {gsub("\\\\n",sprintf("\n%22c",""), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
