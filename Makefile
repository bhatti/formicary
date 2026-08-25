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

# docker-build: build + push multi-arch image to Docker Hub.
#
# Design:
#   - Each arch is built and pushed independently to a platform-specific tag.
#     This isolates failures: a push timeout on amd64 does not require re-building arm64.
#   - Each arch step retries up to 3 times. The session-timeout error ("no active session")
#     is a transient Docker Hub / buildkitd gRPC issue; retrying always succeeds.
#   - The builder is created once and reused across runs (never torn down here).
#     Use `make docker-builder-reset` only when the builder is broken.
#   - The manifest step (`docker-manifest`) is a separate target so it can be re-run
#     independently if only the manifest failed while both arch images are already pushed.
#
# Typical usage:
#   make docker-build              # full build + push + manifest
#   make docker-manifest           # re-run manifest only (both arch tags already on Hub)
#   make docker-build-amd64        # rebuild + push amd64 only
#   make docker-builder-reset      # nuke + recreate the buildx builder

PUSH_RETRIES ?= 3
DOCKER_REGISTRY_PREFIX ?= plexobject

# Retry wrapper: retries a docker buildx build command up to PUSH_RETRIES times.
# Usage: $(call docker_push_with_retry, <buildx args>)
define docker_push_with_retry
	@attempt=1; \
	while [ $$attempt -le $(PUSH_RETRIES) ]; do \
	  echo "  Attempt $$attempt/$(PUSH_RETRIES): docker buildx build $(1)"; \
	  if docker buildx build $(1); then \
	    echo "  Push succeeded on attempt $$attempt."; \
	    break; \
	  fi; \
	  attempt=$$((attempt + 1)); \
	  if [ $$attempt -le $(PUSH_RETRIES) ]; then \
	    echo "  Push failed — retrying in 5s..."; \
	    sleep 5; \
	  else \
	    echo "  Push failed after $(PUSH_RETRIES) attempts." >&2; \
	    exit 1; \
	  fi; \
	done
endef

# Ensure the persistent multiarch builder exists (no-op if already present).
docker-builder:
	@if ! docker buildx inspect multiarch > /dev/null 2>&1; then \
	  echo "Creating buildx builder 'multiarch'..."; \
	  docker buildx create --name multiarch --driver docker-container \
	    --driver-opt network=host --use --bootstrap; \
	else \
	  docker buildx use multiarch; \
	fi

# Remove and recreate the builder. Use only when the builder is broken.
docker-builder-reset:
	docker buildx rm multiarch 2>/dev/null || true
	docker buildx create --name multiarch --driver docker-container \
	  --driver-opt network=host --use --bootstrap
	@echo "Builder 'multiarch' recreated."

# Build + push linux/arm64 only.
docker-build-arm64: docker-builder
	@echo "=== Building linux/arm64 ==="
	$(call docker_push_with_retry, \
	  --platform linux/arm64 \
	  --build-arg APP_VERSION=$(VERSION) \
	  --build-arg CACHEBUST=$(shell date +%s) \
	  --tag $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-arm64 \
	  --provenance=false --push .)
	@echo "=== linux/arm64 pushed: $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-arm64 ==="

# Build + push linux/amd64 only.
docker-build-amd64: docker-builder
	@echo "=== Building linux/amd64 ==="
	$(call docker_push_with_retry, \
	  --platform linux/amd64 \
	  --build-arg APP_VERSION=$(VERSION) \
	  --build-arg CACHEBUST=$(shell date +%s) \
	  --tag $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-amd64 \
	  --provenance=false --push .)
	@echo "=== linux/amd64 pushed: $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-amd64 ==="

# Create (or update) the multi-arch manifest from already-pushed arch tags.
# Safe to re-run if docker-build succeeded for both arches but manifest creation timed out.
docker-manifest:
	@echo "=== Creating multi-arch manifest ==="
	docker buildx imagetools create \
	  -t $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION) \
	  -t $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):latest \
	  $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-arm64 \
	  $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)-amd64
	@echo "=== Multi-arch manifest published: $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION) / latest ==="

# Full build: vendor → build arm64 → build amd64 → manifest.
docker-build: vendor docker-build-arm64 docker-build-amd64 docker-manifest

# docker-push: re-push already-built versioned + latest tags (no rebuild).
docker-push:
	docker push $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):$(VERSION)
	docker push $(DOCKER_REGISTRY_PREFIX)/$(BINARY_NAME):latest

docker-release:
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker tag $(BINARY_NAME) $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)
	# Push the docker images
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):latest
	docker push $(DOCKER_REGISTRY)$(BINARY_NAME):$(VERSION)

DOCKER_IMAGE ?= plexobject/formicary:latest
COMMON_AUTH_GOOGLE_CALLBACK_HOST ?= localhost
COMMON_AUTH_GITHUB_CALLBACK_HOST ?= localhost
# Set COMMON_AUTH_ENABLED=false to disable auth for local testing (no OAuth creds needed).
# For Google OAuth: export COMMON_AUTH_GOOGLE_CLIENT_ID and COMMON_AUTH_GOOGLE_CLIENT_SECRET.
# For GitHub OAuth: export COMMON_AUTH_GITHUB_CLIENT_ID and COMMON_AUTH_GITHUB_CLIENT_SECRET.
# Always set COMMON_AUTH_JWT_SECRET to a stable secret (sessions break if it changes).
# COMMON_QUEUE_TOKEN: ant's API JWT token for authenticating to the queen WebSocket endpoint.
# Generate via the formicary UI (Dashboard → API Tokens) and set on the ant only.
# The queen validates it using COMMON_AUTH_JWT_SECRET — no shared secret needed on the queen.
COMMON_AUTH_ENABLED ?= true
COMMON_QUEUE_TOKEN ?=

DATA_DIR ?= $(HOME)/formicary-data
# Config is mounted from the repo — embedded ant + embedded SeaweedFS + SQLite, no external services needed.
CONFIG_FILE ?= $(PWD)/config/formicary-docker.yaml

# docker-run: queen + embedded ant + embedded SeaweedFS + SQLite in one container.
# No Redis, MinIO, or separate ant container needed.
# Override with a locally-built image: DOCKER_IMAGE=formicary:latest make docker-run
KUBECONFIG_PATCHED ?= $(DATA_DIR)/kubeconfig

# Patch kubeconfig for use inside Docker:
#   - Replace 127.0.0.1 with host.docker.internal (host reachable from container)
#   - Set insecure-skip-tls-verify: true (cert is valid for localhost, not host.docker.internal)
#   - Remove certificate-authority-data (superseded by insecure-skip-tls-verify)
$(KUBECONFIG_PATCHED): $(HOME)/.kube/config
	mkdir -p $(DATA_DIR)
	python3 $(PWD)/scripts/patch-kubeconfig.py $< $@
	chmod 600 $@

# docker-run-queen: queen-only (no embedded ant). Ants connect via WebSocket.
# Override config: CONFIG_FILE=$(PWD)/config/formicary-queen.yaml make docker-run-queen
QUEEN_CONFIG_FILE ?= $(PWD)/config/formicary-queen.yaml

docker-run-queen: $(KUBECONFIG_PATCHED)
	mkdir -p $(DATA_DIR)
	docker run --rm -p 7777:7777 -p 19000:19000 \
		-e COMMON_AUTH_ENABLED="$(COMMON_AUTH_ENABLED)" \
		-e COMMON_AUTH_JWT_SECRET="$(COMMON_AUTH_JWT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_ID="$(COMMON_AUTH_GOOGLE_CLIENT_ID)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_SECRET="$(COMMON_AUTH_GOOGLE_CLIENT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CALLBACK_HOST="$(COMMON_AUTH_GOOGLE_CALLBACK_HOST)" \
		-e COMMON_AUTH_GITHUB_CLIENT_ID="$(COMMON_AUTH_GITHUB_CLIENT_ID)" \
		-e COMMON_AUTH_GITHUB_CLIENT_SECRET="$(COMMON_AUTH_GITHUB_CLIENT_SECRET)" \
		-e COMMON_AUTH_GITHUB_CALLBACK_HOST="$(COMMON_AUTH_GITHUB_CALLBACK_HOST)" \
		-v $(DATA_DIR):/data \
		-v $(QUEEN_CONFIG_FILE):/config/formicary-queen.yaml:ro \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(KUBECONFIG_PATCHED):/home/formicary-user/.kube/config:ro \
		$(DOCKER_IMAGE) \
		--config /config/formicary-queen.yaml

docker-run: $(KUBECONFIG_PATCHED)
	mkdir -p $(DATA_DIR)
	docker run --rm -p 7777:7777 -p 19000:19000 \
		-e COMMON_AUTH_ENABLED="$(COMMON_AUTH_ENABLED)" \
		-e COMMON_AUTH_JWT_SECRET="$(COMMON_AUTH_JWT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_ID="$(COMMON_AUTH_GOOGLE_CLIENT_ID)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_SECRET="$(COMMON_AUTH_GOOGLE_CLIENT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CALLBACK_HOST="$(COMMON_AUTH_GOOGLE_CALLBACK_HOST)" \
		-e COMMON_AUTH_GITHUB_CLIENT_ID="$(COMMON_AUTH_GITHUB_CLIENT_ID)" \
		-e COMMON_AUTH_GITHUB_CLIENT_SECRET="$(COMMON_AUTH_GITHUB_CLIENT_SECRET)" \
		-e COMMON_AUTH_GITHUB_CALLBACK_HOST="$(COMMON_AUTH_GITHUB_CALLBACK_HOST)" \
		-v $(DATA_DIR):/data \
		-v $(CONFIG_FILE):/config/formicary-queen.yaml:ro \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(KUBECONFIG_PATCHED):/home/formicary-user/.kube/config:ro \
		$(DOCKER_IMAGE)

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
	DB_DATA_SOURCE="./formicary_db.sqlite" \
	COMMON_S3_LOCAL_DATA_DIR="./data/seaweedfs" \
	COMMON_S3_LOCAL_WEED_BIN="./bin/weed" \
	COMMON_PUBLIC_DIR="./public/" \
	PATH="$(PWD)/bin:$(PATH)" ./"out/bin/${BINARY_NAME}" --config config/formicary-queen-embedded.yaml

# Queen-only mode — no embedded ant. Ants connect separately via WebSocket.
run-queen: build bin/weed
	DB_DATA_SOURCE="./formicary_db.sqlite" \
	COMMON_S3_LOCAL_DATA_DIR="./data/seaweedfs" \
	COMMON_S3_LOCAL_WEED_BIN="./bin/weed" \
	COMMON_PUBLIC_DIR="./public/" \
	PATH="$(PWD)/bin:$(PATH)" ./"out/bin/${BINARY_NAME}" --config config/formicary-queen.yaml

# Queen + embedded ant + embedded SeaweedFS in Docker (uses formicary-docker.yaml baked into image).
docker-run-embedded: $(KUBECONFIG_PATCHED)
	mkdir -p $(DATA_DIR)
	docker run --rm -p 7777:7777 -p 19000:19000 \
		-e COMMON_AUTH_ENABLED="$(COMMON_AUTH_ENABLED)" \
		-e COMMON_AUTH_JWT_SECRET="$(COMMON_AUTH_JWT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_ID="$(COMMON_AUTH_GOOGLE_CLIENT_ID)" \
		-e COMMON_AUTH_GOOGLE_CLIENT_SECRET="$(COMMON_AUTH_GOOGLE_CLIENT_SECRET)" \
		-e COMMON_AUTH_GOOGLE_CALLBACK_HOST="$(COMMON_AUTH_GOOGLE_CALLBACK_HOST)" \
		-e COMMON_AUTH_GITHUB_CLIENT_ID="$(COMMON_AUTH_GITHUB_CLIENT_ID)" \
		-e COMMON_AUTH_GITHUB_CLIENT_SECRET="$(COMMON_AUTH_GITHUB_CLIENT_SECRET)" \
		-e COMMON_AUTH_GITHUB_CALLBACK_HOST="$(COMMON_AUTH_GITHUB_CALLBACK_HOST)" \
		-v $(DATA_DIR):/data \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(KUBECONFIG_PATCHED):/home/formicary-user/.kube/config:ro \
		$(DOCKER_IMAGE)

# Local ant — connects to queen at localhost:7777 (uses config/formicary-ant.yaml).
ant: build
	COMMON_QUEUE_TOKEN="$(COMMON_QUEUE_TOKEN)" \
	./"out/bin/${BINARY_NAME}" ant --config config/formicary-ant.yaml --id=formicary-ant-id1 --port 7771 --tags "builder pulsar redis kotlin aws-lambda"

# Remote ant — connects to queen via QUEEN_URL env var (default: localhost).
# Override: make ant-remote QUEEN_URL=ws://queen.example.com:7777/ws/queue
QUEEN_URL ?= ws://localhost:7777/ws/queue
QUEEN_S3_ENDPOINT ?= localhost:19000
ANT_ID ?= formicary-ant-remote-1
ANT_TAGS ?= docker shell builder

ant-remote: build
	COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="$(QUEEN_URL)" \
	COMMON_QUEUE_TOKEN="$(COMMON_QUEUE_TOKEN)" \
	COMMON_S3_ENDPOINT="$(QUEEN_S3_ENDPOINT)" \
	COMMON_S3_ACCESS_KEY_ID="localkey" \
	COMMON_S3_SECRET_ACCESS_KEY="localsecret" \
	./"out/bin/${BINARY_NAME}" ant \
		--config config/formicary-ant.yaml \
		--id=$(ANT_ID) \
		--tags "$(ANT_TAGS)"

# Ant in Docker — connects to queen via QUEEN_URL env var.
# Override: make ant-docker QUEEN_URL=ws://queen.example.com:7777/ws/queue
ant-docker:
	docker run --rm \
		--network host \
		-e COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="$(QUEEN_URL)" \
		-e COMMON_QUEUE_TOKEN="$(COMMON_QUEUE_TOKEN)" \
		-e COMMON_S3_ENDPOINT="$(QUEEN_S3_ENDPOINT)" \
		-e COMMON_S3_ACCESS_KEY_ID="localkey" \
		-e COMMON_S3_SECRET_ACCESS_KEY="localsecret" \
		-v $(PWD)/config/formicary-ant.yaml:/config/formicary-ant.yaml:ro \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(DOCKER_IMAGE) \
		ant --config /config/formicary-ant.yaml --id=$(ANT_ID) --tags "$(ANT_TAGS)"


test:
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > junit-report.xml)
endif
	$(GOTEST) -v $(TEST_RACE_PROCESS) ./... $(OUTPUT_OPTIONS)

# deploy-ant: deploy the ant worker to the local k8s cluster pointing at a remote queen.
# Requires: FORMICARY_URL and FORMICARY_TOKEN env vars (same as deploy-ai-workflows.sh).
# Optional overrides: QUEEN_PORT (default: derived from URL), QUEEN_S3_PORT (default: 19000),
#                     BUFFER_DB_PATH (default: :memory:), ANT_IMAGE (default: plexobject/formicary:latest)
# Example:
#   export FORMICARY_URL=https://10.8.97.24.nip.io
#   export FORMICARY_TOKEN=<jwt>
#   make deploy-ant
deploy-ant:
	@if [ -z "$(FORMICARY_URL)" ]; then \
	  echo "Error: FORMICARY_URL is required. Example: export FORMICARY_URL=https://10.8.97.24.nip.io" >&2; \
	  exit 1; \
	fi
	@if [ -z "$(FORMICARY_TOKEN)" ]; then \
	  echo "Error: FORMICARY_TOKEN is required." >&2; \
	  exit 1; \
	fi
	FORMICARY_URL="$(FORMICARY_URL)" \
	FORMICARY_TOKEN="$(FORMICARY_TOKEN)" \
	QUEEN_PORT="$(QUEEN_PORT)" \
	QUEEN_S3_PORT="$(QUEEN_S3_PORT)" \
	BUFFER_DB_PATH="$(BUFFER_DB_PATH)" \
	ANT_IMAGE="$(ANT_IMAGE)" \
	./scripts/setup-ant-worker.sh

# deploy-queen: deploy or restart the Formicary queen on EC2.
# Requires: EC2_IP env var (or --ec2-ip flag).
# Use --restart to pull the latest image and restart without full redeploy.
# Example:
#   export EC2_IP=10.8.97.24
#   make deploy-queen            # full deploy
#   make deploy-queen ARGS=--restart  # pull latest image + restart
deploy-queen:
	@if [ -z "$(EC2_IP)" ]; then \
	  echo "Error: EC2_IP is required. Example: export EC2_IP=10.8.97.24" >&2; \
	  exit 1; \
	fi
	EC2_IP="$(EC2_IP)" ./scripts/deploy-formicary.sh $(ARGS)

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

## gen-tls-certs: Generate self-signed TLS cert+key.
## Includes IP SAN for EC2_IP and nip.io DNS SAN (required for Google OAuth).
## Google OAuth blocks IP redirect URIs — use <ip>.nip.io as the OAuth callback domain.
## Usage:
##   make gen-tls-certs EC2_IP=10.8.97.24
EC2_IP ?= 127.0.0.1
gen-tls-certs:
	mkdir -p certs
	openssl req -x509 -newkey rsa:4096 \
	    -keyout certs/tls.key \
	    -out    certs/tls.crt \
	    -days   365 -nodes \
	    -subj   "/CN=$(EC2_IP).nip.io/O=Formicary/C=US" \
	    -addext "subjectAltName=IP:$(EC2_IP),IP:127.0.0.1,DNS:$(EC2_IP).nip.io,DNS:formicary.local,DNS:localhost"
	@echo ""
	@echo "Certificate: certs/tls.crt"
	@echo "  SANs: IP:$(EC2_IP), IP:127.0.0.1, DNS:$(EC2_IP).nip.io, DNS:localhost"
	@echo ""
	@echo "Google OAuth redirect URI: https://$(EC2_IP).nip.io/auth/google/callback"
	@echo ""
	@echo "Create K8s secret:"
	@echo "  kubectl create secret tls formicary-tls \\"
	@echo "    --cert=certs/tls.crt --key=certs/tls.key"

.PHONY: vendor build test tag-release bump-patch bump-minor bump-major \
        docker-run docker-run-embedded docker-run-queen \
        ant ant-remote ant-docker gen-tls-certs \
        docker-builder docker-builder-reset \
        docker-build docker-build-arm64 docker-build-amd64 docker-manifest docker-push \
        deploy-ant deploy-queen

