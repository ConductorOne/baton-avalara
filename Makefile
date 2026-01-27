GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
BUILD_DIR = dist/${GOOS}_${GOARCH}
GENERATED_CONF = pkg/config/conf.gen.go

ifeq ($(GOOS),windows)
OUTPUT_PATH = ${BUILD_DIR}/baton-avalara.exe
else
OUTPUT_PATH = ${BUILD_DIR}/baton-avalara
endif

.PHONY: generate
generate: $(GENERATED_CONF)

$(GENERATED_CONF): pkg/config/config.go go.mod
	go generate ./pkg/config

.PHONY: build
build: $(GENERATED_CONF) ## Build the baton-avalara binary
	go build -o ${OUTPUT_PATH} ./cmd/baton-avalara

.PHONY: build-debug
build-debug: ## Build the baton-avalara binary with debug symbols
	go build -gcflags="all=-N -l" -o ${OUTPUT_PATH}_debug ./cmd/baton-avalara

.PHONY: update-deps
update-deps:
	go get -d -u ./...
	go mod tidy -v
	go mod vendor

.PHONY: add-dep
add-dep:
	go mod tidy -v
	go mod vendor

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test ./...

.PHONY: test-server
test-server: ## Run the test server
	go run ./cmd/test-server

.PHONY: targets
targets:
	@echo "Available targets:"
	@awk -F ':' '/^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
