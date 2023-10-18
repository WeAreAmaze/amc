BUILD_TIME := $(shell date +"%Y-%m-%d %H:%M:%S")
#GIT_COMMIT := $(shell git show -s --pretty=format:%h)
GO_VERSION := $(shell go version)
BUILD_PATH := ./build/bin/
APP_NAME := amazechain
APP_PATH := ./cmd/amc
SHELL := /bin/bash
#LDFLAGS := -ldflags "-w -s -X github.com/amazechain/amc/version.BuildNumber=${GIT_COMMIT} -X 'github.com/amazechain/amc/version.BuildTime=${BUILD_TIME}' -X 'github.com/amazechain/amc/version.GoVersion=${GO_VERSION}'"

GIT_COMMIT ?= $(shell git rev-list -1 HEAD)
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
GIT_TAG    ?= $(shell git describe --tags '--match=v*' --dirty)
PACKAGE = github.com/amazechain/amc
GO_FLAGS += -ldflags "-X ${PACKAGE}/params.GitCommit=${GIT_COMMIT} -X ${PACKAGE}/params.GitBranch=${GIT_BRANCH} -X ${PACKAGE}/params.GitTag=${GIT_TAG}"
GOBUILD = go build -v $(GO_FLAGS)


# if using volume-mounting data dir, then must exist on host OS
DOCKER_UID ?= $(shell id -u)
DOCKER_GID ?= $(shell id -g)


# == mobiles
#OSFLAG=$(shell uname -sm)

ANDROID_SDK=$(ANDROID_HOME)
NDK_VERSION=21.1.6352462
NDK_HOME=$(ANDROID_SDK)/ndk/$(NDK_VERSION)
#ANDROID_SDK=/Users/mac/Library/Android/sdk
MOBILE_GO_FLAGS = -ldflags "-X ${PACKAGE}/cmd/evmsdk/common.VERSION=${GIT_COMMIT}"
MOBILE_PACKAGE= $(shell pwd)/cmd/evmsdk
BUILD_MOBILE_PATH = ./build/mobile/


# --build-arg UID=${DOCKER_UID} --build-arg GID=${DOCKER_GID}

## go-version:                        print and verify go version
go-version:
	@if [ $(shell go version | cut -c 16-17) -lt 18 ]; then \
		echo "minimum required Golang version is 1.18"; \
		exit 1 ;\
	fi
gen:
	@echo "Generate go code ..."
	go generate ./...
	@echo "Generate done!"
deps: go-version
	@echo "setup go deps..."
	go mod tidy
	@echo "deps done!"

amc: deps
	@echo "start build $(APP_NAME)..."
	#go build -v ${LDFLAGS} -o $(BUILD_PATH)$(APP_NAME)  ${APP_PATH}
	$(GOBUILD) -o $(BUILD_PATH)$(APP_NAME)  ${APP_PATH}
	@echo "Compile done!"

images:
	@echo "docker images build ..."
	DOCKER_BUILDKIT=1 docker build -t amazechain/amc:local .
	@echo "Compile done!"

up:
	@echo "docker compose up $(APP_NAME) ..."
	docker-compose  --project-name $(APP_NAME) up -d
	docker-compose  --project-name $(APP_NAME) logs -f
down:
	@echo "docker compose down $(APP_NAME) ..."
	docker-compose  --project-name $(APP_NAME) down
	docker volume ls -q | grep 'amazechain' | xargs -I % docker volume rm %
	@echo "done!"
stop:
	@echo "docker compose stop $(APP_NAME) ..."
	docker-compose  --project-name $(APP_NAME) stop
	@echo "done!"
start:
	@echo "docker compose stop $(APP_NAME) ..."
	docker-compose  --project-name $(APP_NAME) start
	docker-compose  --project-name $(APP_NAME) logs -f
clean:
	go clean
	@rm -rf  build

devtools:
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	env GOBIN= go install github.com/prysmaticlabs/fastssz/sszgen@latest
	env GOBIN= go install github.com/prysmaticlabs/protoc-gen-go-cast@latest

devimg:
	@echo "docker dev images build ..."
	DOCKER_BUILDKIT=1 docker build -f Dockerfile.dev -t amazechain/amc:devbase .
	@echo "Compile done!"
dev:
	@mkdir -p $(HOME)/.metachain
	go run ./cmd/amc --data.dir=$(HOME)/.metachain/ --log.level=debug --http --http.port=20012 --http.addr=0.0.0.0 --ws --ws.port=20013 --ws.addr=0.0.0.0 \
	--engine.miner --engine.etherbase=0x588639773bc6f163aa262245cda746c120676431 --engine.type=APosEngine \
	--log.level debug \
	--account.unlock=0x588639773bc6f163aa262245cda746c120676431 --account.allow.insecure.unlock --account.password $(HOME)/.metachain/passwd

PACKAGE_NAME          := github.com/WeAreAmaze/amc
GOLANG_CROSS_VERSION  ?= v1.20.7

.PHONY: release
release:
	@docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e GITHUB_TOKEN \
		-e DOCKER_USERNAME \
		-e DOCKER_PASSWORD \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		--clean --skip-validate

		@docker image push --all-tags amazechain/amc


#== mobiles start
mobile: clean mobile-dir android ios

mobile-dir:
	#go get golang.org/x/mobile/bind/objc
	mkdir -p $(BUILD_MOBILE_PATH)/android
ios:
	ANDROID_HOME=$(ANDROID_SDK) ANDROID_NDK_HOME=$(NDK_HOME) gomobile bind ${MOBILE_GO_FLAGS} -androidapi 21 -o $(BUILD_MOBILE_PATH)/evmsdk.xcframework -target=ios $(MOBILE_PACKAGE)
android:
	ANDROID_HOME=$(ANDROID_SDK) ANDROID_NDK_HOME=$(NDK_HOME) gomobile bind -x ${MOBILE_GO_FLAGS} -androidapi 21 -o $(BUILD_MOBILE_PATH)/android/evmsdk.aar -target=android $(MOBILE_PACKAGE)

open-output:
	open ./mobile

#== mobiles end