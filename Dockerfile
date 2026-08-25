ARG WEED_VERSION=3.68
ARG APP_VERSION=0.1.0

FROM golang:1.26 AS go-builder
ARG APP_VERSION
# CGO is required for mattn/go-sqlite3. Install gcc and sqlite headers.
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev libsqlite3-dev && rm -rf /var/lib/apt/lists/*

ARG WEED_VERSION=3.68
RUN ARCH=$(dpkg --print-architecture) && \
    for i in 1 2 3; do \
        curl -fsSL --retry 5 --retry-delay 5 --retry-max-time 120 \
            "https://github.com/seaweedfs/seaweedfs/releases/download/${WEED_VERSION}/linux_${ARCH}.tar.gz" \
            | tar -xz -C /usr/local/bin weed && break; \
        echo "seaweedfs download attempt $i failed, retrying..."; sleep 10; \
    done && \
    chmod +x /usr/local/bin/weed

ARG GOOSE_VERSION=3.17.0
RUN GOARCH=$(go env GOARCH) && \
    case "$GOARCH" in amd64) GOOSE_ARCH="x86_64" ;; arm64) GOOSE_ARCH="arm64" ;; *) GOOSE_ARCH="$GOARCH" ;; esac && \
    for i in 1 2 3; do \
        curl -fsSL --retry 5 --retry-delay 5 --retry-max-time 120 \
            "https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_${GOOSE_ARCH}" \
            -o /usr/local/bin/goose && break; \
        echo "goose download attempt $i failed, retrying..."; sleep 10; \
    done && \
    chmod +x /usr/local/bin/goose

# CACHEBUST must come AFTER binary downloads so those layers stay cached.
# Pass --build-arg CACHEBUST=$(date +%s) to force a fresh code copy + rebuild.
ARG CACHEBUST=1
COPY . /src
WORKDIR /src

ENV CGO_ENABLED=1
RUN mkdir -p out/bin && \
    go build -mod=vendor \
    -ldflags "-X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%S) -X main.version=${APP_VERSION}" \
    -o out/bin/formicary . || (echo "Build failed"; exit 1)

# Production stage
FROM debian:bookworm-slim
RUN rm -rf /var/lib/apt/lists/* && \
    apt-get clean && \
    apt-get update --allow-releaseinfo-change && \
    apt-get install -y --no-install-recommends \
    ca-certificates bash graphviz default-mysql-client postgresql-client && \
    rm -rf /var/lib/apt/lists/* && \
    addgroup --system formicary-user && \
    adduser --system --ingroup formicary-user formicary-user

COPY --from=go-builder /src/out/bin/formicary /formicary
COPY --from=go-builder /usr/local/bin/goose /usr/local/bin/goose
COPY --from=go-builder /usr/local/bin/weed /usr/local/bin/weed
RUN mkdir -p /app/public
COPY --from=go-builder /src/public /app/public
COPY --from=go-builder /src/migrations /migrations
COPY migrations/migrate.sh /usr/local/bin/migrate.sh
RUN mkdir -p /config
COPY --from=go-builder /src/config/formicary-docker.yaml /config/formicary-queen.yaml

RUN chmod +x /usr/local/bin/migrate.sh /usr/local/bin/goose /formicary && \
    /usr/local/bin/goose --version

RUN mkdir -p /data /app/data /tmp/formicary /var/log/formicary /home/formicary-user/.kube && \
    chown -R formicary-user:formicary-user /data /app /tmp/formicary /var/log/formicary /home/formicary-user && \
    chmod 755 /data /app/data /tmp/formicary /var/log/formicary && \
    chmod 700 /home/formicary-user/.kube

ENV HOME="/home/formicary-user" \
    DB_NAME="formicary_db" \
    DB_USER="formicary_user" \
    DB_HOST="localhost" \
    DB_PORT="5432" \
    DB_TYPE="sqlite" \
    DB_SSL_MODE="disable" \
    PUBLIC_DIR="/app/public/" \
    CONFIG_FILE="/config/formicary-queen.yaml" \
    DATA_DIR="/data"

WORKDIR /app
USER formicary-user
ENTRYPOINT ["/bin/bash", "-c", "/usr/local/bin/migrate.sh && exec /formicary --config $CONFIG_FILE"]
