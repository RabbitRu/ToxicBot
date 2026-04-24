GO?=go
GOPATH?=$(shell go env GOPATH)
GOPACKAGES=$(shell go list ./...)
GOLANGCI_LINT_VERSION ?= v2.11.3
GOLANGCI_LINT_DOCKER = docker run --rm \
	-v $(CURDIR):/app \
	-w /app \
	golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)

### билдит докер образ для выравнивания структур
align-build:
	DOCKER_SCAN_SUGGEST=false docker build -t golang-align-check --no-cache - < $(PWD)/tools/align.Dockerfile

### выравнивает структуры для меньшей аллокации
align:
	docker run --rm -v $(PWD):/app -w /app golang-align-check fieldalignment -fix ./...

### врубает линтер
lint:
	$(GOLANGCI_LINT_DOCKER) golangci-lint run ./...

### выравнивает импорты
imports:
	docker run --rm -v $(pwd):/data cytopia/goimports -d .

fmt:
	$(GOLANGCI_LINT_DOCKER) golangci-lint fmt ./...

migration:
	docker run -v $(PWD)/db/migrations:/migrations migrate/migrate:v4.18.3 create -ext sql -dir /migrations -seq $(name)

dozzlepwd:
	docker run --rm httpd:alpine htpasswd -bnBC 10 "" $(password) | tr -d ':\n'