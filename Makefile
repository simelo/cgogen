.DEFAULT_GOAL := help
.PHONY: run build

build:
	mkdir -p ./build ./bin
	cd ./build && go build ../src/cmd/cgogen.go
	mv ./build/cgogen $(GOPATH)/bin/cgogen

run:      ## Run the skycoin node. To add arguments, do 'make ARGS="--foo" run'.
	go run src/cmd/cgogen.go ${ARGS}

