.DEFAULT_GOAL := help
.PHONY: run build lint

# Compilation output
.ONESHELL:
SHELL := /bin/bash

MKFILE_PATH   = $(abspath $(lastword $(MAKEFILE_LIST)))
REPO_ROOT     = $(dir $(MKFILE_PATH))
LIBSRC_DIR = $(REPO_ROOT)/src/cmd/
LIB_FILES = $(shell find $(LIBSRC_DIR) -type f -name "*.go")

build: ## Build cmd cgogen
	rm -rfv $(GOPATH)/bin/cgogen
	go build -o $(GOPATH)/bin/cgogen  $(LIB_FILES)

run:      ## Run the skycoin node. To add arguments, do 'make ARGS="--foo" run'.
	go run src/cmd/cgogen.go ${ARGS}

lint: ## Run linters. Use make install-linters first.
#	vendorcheck ./...
	# lib/cgo needs separate linting rules
	golangci-lint run -c .golangci.yml $(REPO_ROOT)/src/cmd/...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately
#	go vet -all ./...

install-linters: ## Install linters
	go get -u github.com/FiloSottile/vendorcheck
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(shell go env GOPATH)/bin v1.18.0

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'