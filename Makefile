GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=formicary
BRANCH=$(shell git rev-parse --symbolic-full-name --abbrev-ref HEAD)
COMMIT?=$(shell git describe --always --long --dirty)
VERSION_MAJOR?=0
VERSION_MINOR?=1
VERSION_PATCH?=$(shell git rev-list --count HEAD 2>/dev/null || echo 0)
VERSION?=$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)
DATE?=$(shell date -u '+%Y-%m-%dT%H:%M:%S')
SERVICE_PORT?=3000
#TEST_RACE_PROCESS=-race
TEST_RACE_PROCESS=-p 1
PKG_LIST=$(shell go list ./... | grep -v /vendor/)
EXPORT_RESULT?=false # for CI please set EXPORT_RESULT to true

all: test vendor build

build: proto vendor
	mkdir -p out/bin
	$(GOCMD) build -mod vendor -ldflags "-X main.commit=$(COMMIT) -X main.date=$(DATE) -X main.version=$(VERSION)" -o out/bin/$(BINARY_NAME) .

#CGO_ENABLED=1 GOOS=linux GOARCH=amd64 
build-linux:
	@echo "Running build-linux..."
	@echo "GOCMD=$(GOCMD)"
	@echo "COMMIT=$(COMMIT)"
	@echo "DATE=$(DATE)"
	@echo "VERSION=$(VERSION)"
	@echo "Output binary will be: out/bin/$(BINARY_NAME)"
	mkdir -p out/bin
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
	$(GOCMD) build -mod=mod \
	-ldflags "-X main.commit=$(COMMIT) -X main.date=$(DATE) -X main.version=$(VERSION)" \
	-o "out/bin/$(BINARY_NAME)" -v

build-linux-static: vendor
	mkdir -p out/bin
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	CGO_CFLAGS="-D_LARGEFILE64_SOURCE" \
	CGO_LDFLAGS="-static" \
	$(GOCMD) build -mod=mod \
	-ldflags "-X main.commit=$(COMMIT) -X main.date=$(DATE) -X main.version=$(VERSION) -w -s -extldflags '-static'" \
	-o "out/bin/$(BINARY_NAME)" -v

clean:
	rm -fr ./bin
	rm -fr ./out
	rm -fr ./vendor
	rm -f ./junit-report.xml checkstyle-report.xml ./coverage.xml ./profile.cov yamllint-checkstyle.xml

coverage:
	$(GOTEST) -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/AlekSi/gocov-xml
	GO111MODULE=off go get -u github.com/axw/gocov/gocov
	gocov convert profile.cov | gocov-xml > coverage.xml
endif


PROTO_DIR=proto
GEN_DIR=gen

.PHONY: proto lint-proto openapi clean-proto

lint-proto:
	cd $(PROTO_DIR) && buf lint

proto: lint-proto
	cd $(PROTO_DIR) && buf generate

openapi: proto
	@cp $(GEN_DIR)/openapi/formicary.swagger.json $(GEN_DIR)/openapi/openapi.json
	@cp $(GEN_DIR)/openapi/formicary.swagger.json public/docs/openapi.json
	@echo "OpenAPI spec: $(GEN_DIR)/openapi/openapi.json + public/docs/openapi.json"

clean-proto:
	# Remove generated proto files only (*.pb.go, *_grpc.pb.go, *.pb.gw.go, swagger json).
	# Hand-written _ext.go files are left untouched.
	find $(GEN_DIR)/go -name "*.pb.go" -o -name "*_grpc.pb.go" -o -name "*.pb.gw.go" | xargs rm -f
	rm -f $(GEN_DIR)/openapi/formicary.swagger.json $(GEN_DIR)/openapi/openapi.json public/docs/openapi.json

docker-build:
	docker build --rm --build-arg APP_VERSION=$(VERSION) --tag $(BINARY_NAME):$(VERSION) --tag $(BINARY_NAME):latest .

docker-release:
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)
	# Push the docker images
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)

lint: 
	golangci-lint run --enable-all

vet: clean 
	$(GOVET) ./... 2> go-vet-report.out

WEED_VERSION ?= 3.68
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Darwin)
  ifeq ($(UNAME_M),arm64)
    WEED_ARCH := darwin_arm64
  else
    WEED_ARCH := darwin_amd64
  endif
else
  WEED_ARCH := linux_amd64
endif

bin/weed:
	mkdir -p bin
	curl -fsSL "https://github.com/seaweedfs/seaweedfs/releases/download/$(WEED_VERSION)/$(WEED_ARCH).tar.gz" \
	    | tar -xz -C bin weed
	chmod +x bin/weed

download-weed: bin/weed

run: build bin/weed
	PATH="$(PWD)/bin:$(PATH)" ./"out/bin/${BINARY_NAME}" --config config/formicary-queen-embedded.yaml

run-queen: build bin/weed
	PATH="$(PWD)/bin:$(PATH)" ./"out/bin/${BINARY_NAME}" --config config/formicary-queen.yaml

ant: build
	./"out/bin/${BINARY_NAME}" ant --config config/formicary-ant.yaml --id=formicary-ant-id1 --port 7771 --tags "builder pulsar redis kotlin aws-lambda"


test:
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > junit-report.xml)
endif
	$(GOTEST) -v $(TEST_RACE_PROCESS) ./... $(OUTPUT_OPTIONS)

vendor:
	$(GOCMD) mod vendor

tag-release: build
	@echo "╔══════════════════════════════════════════════════════════╗"
	@echo "║  Formicary Release Tagging                              ║"
	@echo "╚══════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "  Version : v$(VERSION)"
	@echo "  Commit  : $(COMMIT)"
	@echo "  Date    : $(DATE)"
	@echo ""
	@echo "Run these git commands to complete the release:"
	@echo ""
	@echo "  git add -A"
	@echo "  git commit -m \"chore: release v$(VERSION)\""
	@echo "  git tag -a v$(VERSION) -m \"Release v$(VERSION)\""
	@echo "  git push && git push --tags"
	@echo ""

bump-patch:
	@$(MAKE) tag-release

bump-minor:
	@NEW_MINOR=$$(($(VERSION_MINOR) + 1)); \
	sed -i.bak "s/^VERSION_MINOR?=.*/VERSION_MINOR?=$$NEW_MINOR/" Makefile && rm -f Makefile.bak; \
	echo "Bumped VERSION_MINOR to $$NEW_MINOR in Makefile"
	@$(MAKE) tag-release

bump-major:
	@NEW_MAJOR=$$(($(VERSION_MAJOR) + 1)); \
	sed -i.bak "s/^VERSION_MAJOR?=.*/VERSION_MAJOR?=$$NEW_MAJOR/" Makefile && rm -f Makefile.bak; \
	sed -i.bak 's/^VERSION_MINOR?=.*/VERSION_MINOR?=0/' Makefile && rm -f Makefile.bak; \
	echo "Bumped VERSION_MAJOR to $$NEW_MAJOR, reset VERSION_MINOR to 0 in Makefile"
	@$(MAKE) tag-release

.PHONY: vendor build test tag-release bump-patch bump-minor bump-major

