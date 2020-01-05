.DEFAULT_GOAL := help
.PHONY: run build

# Compilation output
.ONESHELL:
SHELL := /bin/bash

MKFILE_PATH   = $(abspath $(lastword $(MAKEFILE_LIST)))
REPO_ROOT     = $(dir $(MKFILE_PATH))
LIBSRC_DIR = $(REPO_ROOT)/src/cmd/
LIB_FILES = $(shell find $(LIBSRC_DIR) -type f -name "*.go")

build:
	go build -o $(GOPATH)/bin/cgogen  $(LIB_FILES)

run:      ## Run the skycoin node. To add arguments, do 'make ARGS="--foo" run'.
	go run src/cmd/cgogen.go ${ARGS}

