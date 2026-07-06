ARG WEED_VERSION=3.68
ARG APP_VERSION=0.1.0

FROM golang:1.26 AS go-builder
ARG APP_VERSION
COPY . /src
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make bash build-essential \
    graphviz libgraphviz-dev pkg-config \
    ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

ARG WEED_VERSION=3.68
RUN curl -fsSL "https://github.com/seaweedfs/seaweedfs/releases/download/${WEED_VERSION}/linux_amd64.tar.gz" \
    | tar -xz -C /usr/local/bin weed && \
    chmod +x /usr/local/bin/weed

# CGO required for go-graphviz
ENV CGO_ENABLED=1
RUN go mod download && \
    mkdir -p out/bin && \
    go build -mod=mod \
    -ldflags "-X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%S) -X main.version=${APP_VERSION}" \
    -o out/bin/formicary . || (echo "Build failed"; exit 1)

# Install Goose — download pre-built binary to avoid a separate go install network fetch
ARG GOOSE_VERSION=3.17.0
RUN ARCH=$(go env GOARCH) && \
    curl -fsSL "https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_${ARCH}" \
         -o /usr/local/bin/goose && \
    chmod +x /usr/local/bin/goose

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

# Copy binaries from builder stage
COPY --from=go-builder /src/out/bin/formicary /formicary
COPY --from=go-builder /usr/local/bin/goose /usr/local/bin/goose
COPY --from=go-builder /usr/local/bin/weed /usr/local/bin/weed
# Copy application files
RUN mkdir -p /app/public
COPY --from=go-builder /src/public /app/public
COPY --from=go-builder /src/migrations /migrations

# Copy and make migration script executable
COPY migrations/migrate.sh /usr/local/bin/migrate.sh

# Bake in the default config (embedded ant + embedded SeaweedFS + SQLite).
# No volume mount required — just run: docker run -p 7777:7777 formicary:latest
RUN mkdir -p /config
COPY --from=go-builder /src/config/formicary-docker.yaml /config/formicary-queen.yaml

RUN chmod +x /usr/local/bin/migrate.sh /usr/local/bin/goose /formicary && \
    /usr/local/bin/goose --version

# Create necessary directories
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
    # Auth env vars (COMMON_AUTH_JWT_SECRET, COMMON_AUTH_GOOGLE_CLIENT_ID, etc.)
    # must be supplied at runtime — never bake secrets or auth defaults into the image.

WORKDIR /app
USER formicary-user
ENTRYPOINT ["/bin/bash", "-c", "/usr/local/bin/migrate.sh && exec /formicary --config $CONFIG_FILE"]
