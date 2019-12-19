.DEFAULT_GOAL := help
.PHONY: run build lint install-linters

build:
	mkdir -p ./build ./bin
	cd ./build && go build ../src/cmd/cgogen.go
	mv ./build/cgogen bin/cgogen

run:      ## Run the skycoin node. To add arguments, do 'make ARGS="--foo" run'.
	go run src/cmd/cgogen.go ${ARGS}

lint: ## Run linters. Use make install-linters first.
	vendorcheck ./...
	# lib/cgo needs separate linting rules
	golangci-lint run -c .golangci.yml ./src/...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately
	go vet -all ./...

install-linters: ## Install linters
	go get -u github.com/FiloSottile/vendorcheck
	cat ./ci-scripts/install-golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.10.2

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'