COMMIT_SHA ?= $(shell git rev-parse HEAD)
REPONAME ?= signoz
IMAGE_NAME ?= "signoz/collector"
CONFIG_FILE ?= ./config/default-config.yaml
DOCKER_TAG ?= latest

.PHONY: test
test:
	go test -count=1 -v -race -cover ./...
	cd receiver/signozspanmetricsaprocessor && go test ./...
	cd exporter/clickhousemetricsexporter && go test ./...
	cd exporter/clickhoustracesexporter && go test ./...

.PHONY: build
build:
	$(if $(GOOS),GOOS=${GOOS},) go build ./cmd/signozcollector

.PHONY: run
run:
	go run cmd/signozcollector/* --config ${CONFIG_FILE}

.PHONY: build-push-signozcollector
build-push-signozcollector:
	@echo "------------------"
	@echo "--> Building and pushing otelcontribcol docker image"
	@echo "------------------"
	docker buildx build --platform linux/amd64,linux/arm64 --progress plane \
		--no-cache --push -f cmd/signozcollector/Dockerfile \
		--tag $(REPONAME)/$(IMAGE_NAME):$(DOCKER_TAG) .

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./... && echo "Done."
