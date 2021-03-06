BUILD_FILES =  $(shell go list -f '{{range .GoFiles}}{{$$.Dir}}/{{printf "%s\n" .}}{{end}}' ./...)
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null)
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null)

GO_LDFLAGS := -X main.version=$(VERSION)
GO_LDFLAGS += -X main.revision=$(REVISION)

DOTENV := ../.tmp/.env
PROFILE :=

all: build

build: bin/g2l

bin/g2l: $(BUILD_FILES)
	@go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$@" ./cmd/g2l

lint:
	@echo "golint running..."
	@golint ./...
	@echo "go vet running..."
	@go vet ./...

deploy:
	cd cdk; ./run.sh $(DOTENV) $(PROFILE)
