GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)
DOCKER_COMPOSE=$(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; fi)

ifeq ($(GOHOSTOS), windows)
	#the `find.exe` is different from `find` in bash/shell.
	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	#Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: run
# run
run:
	kratos run

.PHONY: all
# generate all
all:
	make api;
	make config;
	make generate;

.PHONY: docker-build
# docker build image (pass LLM_API_KEY, JWT_SECRET, WECHAT_TOKEN as env vars)
docker-build:
	docker build \
		--build-arg LLM_API_KEY=$(LLM_API_KEY) \
		--build-arg JWT_SECRET=$(JWT_SECRET) \
		--build-arg WECHAT_TOKEN=$(WECHAT_TOKEN) \
		-t seas:latest .

.PHONY: docker-run
# docker run single container (no redis, no data persistence, quick verification only)
docker-run:
	docker run --rm -p 8000:8000 seas:latest

.PHONY: docker-compose-up
# docker compose up (full environment with redis)
docker-compose-up:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Docker Compose is not installed. Install Compose v2 so 'docker compose version' works, or install legacy 'docker-compose'."; \
		exit 1; \
	fi
	@if [ ! -f .env ]; then \
		echo ".env file is missing. Copy .env.example to .env and fill in LLM_API_KEY, JWT_SECRET, and WECHAT_TOKEN."; \
		exit 1; \
	fi
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop or your Docker service, then retry."; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) up -d --build

.PHONY: docker-compose-down
# docker compose down
docker-compose-down:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Docker Compose is not installed. Install Compose v2 so 'docker compose version' works, or install legacy 'docker-compose'."; \
		exit 1; \
	fi
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop or your Docker service, then retry."; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) down

.PHONY: docker-clean
# docker clean images and volumes
docker-clean:
	@if [ -z "$(DOCKER_COMPOSE)" ]; then \
		echo "Docker Compose is not installed. Install Compose v2 so 'docker compose version' works, or install legacy 'docker-compose'."; \
		exit 1; \
	fi
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop or your Docker service, then retry."; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) down -v --remove-orphans
	docker rmi seas:latest 2>/dev/null || true

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
