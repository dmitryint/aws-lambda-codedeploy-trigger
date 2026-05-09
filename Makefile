LAMBDA_HANDLER := bootstrap
BIN_DIR        := .bin
PKG_DIR        := package
CMD_DIR        := cmd/codedeploy-trigger

TAG       := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
BRANCH    := $(if $(TAG),$(TAG),$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null))
HASH      := $(shell git rev-parse --short=7 HEAD 2>/dev/null)
TIMESTAMP := $(shell git log -1 --format=%ct HEAD 2>/dev/null | xargs -I{} date -u -r {} +%Y%m%dT%H%M%S)
GIT_REV   := $(shell printf "%s-%s-%s" "$(BRANCH)" "$(HASH)" "$(TIMESTAMP)")
REV       := $(if $(filter --,$(GIT_REV)),latest,$(GIT_REV))

LDFLAGS := -ldflags "-X main.revision=$(REV) -s -w"

.PHONY: all build build-amd64 build-arm64 package package-amd64 package-arm64 test fmt vet tidy clean

all: package

build:
	cd $(CMD_DIR) && CGO_ENABLED=0 go build -mod=vendor $(LDFLAGS) -o ../../$(BIN_DIR)/codedeploy-trigger

build-amd64:
	mkdir -p $(BIN_DIR)/linux_amd64
	cd $(CMD_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor $(LDFLAGS) -o ../../$(BIN_DIR)/linux_amd64/$(LAMBDA_HANDLER)

build-arm64:
	mkdir -p $(BIN_DIR)/linux_arm64
	cd $(CMD_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -mod=vendor $(LDFLAGS) -o ../../$(BIN_DIR)/linux_arm64/$(LAMBDA_HANDLER)

package: package-amd64 package-arm64

package-amd64: build-amd64
	mkdir -p $(PKG_DIR)
	cd $(BIN_DIR)/linux_amd64 && zip -9 -r ../../$(PKG_DIR)/lambda-codedeploy-trigger_linux_amd64.zip $(LAMBDA_HANDLER)

package-arm64: build-arm64
	mkdir -p $(PKG_DIR)
	cd $(BIN_DIR)/linux_arm64 && zip -9 -r ../../$(PKG_DIR)/lambda-codedeploy-trigger_linux_arm64.zip $(LAMBDA_HANDLER)

test:
	go test -mod=vendor -race -count=1 -timeout=60s ./...

fmt:
	go fmt ./...

vet:
	go vet -mod=vendor ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR) $(PKG_DIR)
