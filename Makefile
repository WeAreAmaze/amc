BUILD_TIME := $(shell date +"%Y-%m-%d %H:%M:%S")
GIT_COMMIT := $(shell git show -s --pretty=format:%h)
GO_VERSION := $(shell go version)
BUILD_PATH := ./build/bin/
APP_NAME := AmazeChain
APP_PATH := ./cmd/amc
SHELL := /bin/bash
LDFLAGS := -ldflags "-w -s -X github.com/amazechain/amc/version.BuildNumber=${GIT_COMMIT} -X 'github.com/amazechain/amc/version.BuildTime=${BUILD_TIME}' -X 'github.com/amazechain/amc/version.GoVersion=${GO_VERSION}'"


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
	go build -v ${LDFLAGS} -o $(BUILD_PATH)$(APP_NAME)  ${APP_PATH}
	@echo "Compile done!"

images:
	@echo "docker images build ..."
	DOCKER_BUILDKIT=1 docker build -t amazechain/amc:latest .
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
