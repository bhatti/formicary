ARG WEED_VERSION=3.68

FROM golang:1.26 AS go-builder
COPY . /src
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make bash build-essential \
    ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

ARG WEED_VERSION=3.68
RUN curl -fsSL "https://github.com/seaweedfs/seaweedfs/releases/download/${WEED_VERSION}/linux_amd64.tar.gz" \
    | tar -xz -C /usr/local/bin weed && \
    chmod +x /usr/local/bin/weed

# Pure Go build (modernc.org/sqlite — no CGO required)
ENV CGO_ENABLED=0
RUN go mod download && \
    mkdir -p out/bin && \
    GOOS=linux GOARCH=amd64 go build -mod=mod \
    -ldflags "-X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%S) -X main.version=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o out/bin/formicary . || (echo "Build failed"; exit 1)

# Install Goose
RUN GOARCH=$(go env GOARCH) GOOS=$(go env GOOS) go install github.com/pressly/goose/v3/cmd/goose@v3.17.0

# Production stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates bash mysql-client postgresql-client && \
    addgroup -S formicary-user && \
    adduser -S -G formicary-user formicary-user

# Copy binaries from builder stage
COPY --from=go-builder /src/out/bin/formicary /formicary
COPY --from=go-builder /go/bin/goose /usr/local/bin/goose
COPY --from=go-builder /usr/local/bin/weed /usr/local/bin/weed
# Copy application files
RUN mkdir -p /app/public
COPY --from=go-builder /src/public /app/public
COPY --from=go-builder /src/migrations /migrations

# Copy and make migration script executable
COPY migrations/migrate.sh /usr/local/bin/migrate.sh

RUN chmod +x /usr/local/bin/migrate.sh /usr/local/bin/goose /formicary && \
    /usr/local/bin/goose --version || (echo "Goose not working, trying to install in Alpine..."; \
    apk add --no-cache go git && \
    go install github.com/pressly/goose/v3/cmd/goose@v3.17.0 && \
    cp /root/go/bin/goose /usr/local/bin/goose && \
    chmod +x /usr/local/bin/goose && \
    apk del go git)

# Create necessary directories
RUN mkdir -p /data /app/data /tmp/formicary /var/log/formicary && \
    chown -R formicary-user:formicary-user /data /app /tmp/formicary /var/log/formicary && \
    chmod 755 /data /app/data /tmp/formicary /var/log/formicary

ENV DB_NAME="formicary_db" \
    DB_USER="formicary_user" \
    DB_HOST="localhost" \
    DB_PORT="5432" \
    DB_TYPE="sqlite" \
    DB_SSL_MODE="disable" \
    PUBLIC_DIR="/public" \
    CONFIG_FILE="/config/formicary-queen.yaml" \
    DATA_DIR="/data"

WORKDIR /app
USER formicary-user
ENTRYPOINT ["/bin/bash", "-c", "/usr/local/bin/migrate.sh && exec /formicary --config $CONFIG_FILE"]
