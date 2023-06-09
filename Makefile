all: build

.PHONY: lint
lint: .prepare ## Lint the files
	@go mod tidy
	@golint ./...
	@golangci-lint run ./...

.PHONY: fix
fix: .prepare ## Lint and fix vialoations
	@go mod tidy
	@go fmt ./...
	@golangci-lint run --fix ./...

.PHONY: mock
mock: .prepare ## Generate test mock interfaces
	@mockery --dir api --name Client
	@mockery --dir persistence --name User
	@mockery --dir persistence --name ChatSession

.PHONY: test
test: .prepare ## Run unittests
	@go test --count 1 -timeout 30s -short ./...

.PHONY: build
build: lint ## Build the application
	@go build -o gpt .

.PHONY: openapi
openapi: .prepare ## Generate the OpenAPI spec
	@swag init -g main.go --parseDependency
	@rm docs/docs.go

.prepare: ## Prepare the project for local development
	@pip3 install --user pre-commit
	@pre-commit install
	@pre-commit install-hooks
	@GO111MODULE=on go install -v github.com/go-critic/go-critic/cmd/gocritic@latest
	@GO111MODULE=on go get -v -u github.com/swaggo/swag/cmd/swag
	@touch .prepare

help: ## Display this help screen
	@grep -h -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
