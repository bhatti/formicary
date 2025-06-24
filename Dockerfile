FROM golang:1.24 AS go-builder

COPY . /src
WORKDIR /src

# Install ALL the static linking dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make bash build-essential \
    sqlite3 libsqlite3-dev pkg-config \
    ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Set CGO flags for static linking
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV CGO_LDFLAGS="-static -w -s"

# Download dependencies and build
RUN go mod download && make build-linux || (echo "Build failed"; exit 1)


# Clear static flags for Goose install
ENV CGO_LDFLAGS=""

# Install Goose in the builder stage
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.17.0

# Production stage
FROM alpine:latest

# Minimal runtime (static binary needs almost nothing)
RUN apk add --no-cache ca-certificates bash mysql-client postgresql-client && \
    addgroup -S formicary-user && \
    adduser -S -G formicary-user formicary-user

# Copy binaries from builder stage
COPY --from=go-builder /src/out/bin/formicary /formicary
COPY --from=go-builder /go/bin/goose /usr/local/bin/goose

# Copy application files
COPY --from=go-builder /src/public /public
COPY --from=go-builder /src/migrations /migrations

# Copy and make migration script executable
COPY migrations/migrate.sh /usr/local/bin/migrate.sh
RUN chmod +x /usr/local/bin/migrate.sh

# Set environment variables
ENV DB_NAME="formicary_db" \
    DB_USER="formicary_user" \
    DB_HOST="localhost" \
    DB_PORT="5432" \
    DB_TYPE="sqlite" \
    DB_SSL_MODE="disable" \
    PUBLIC_DIR="/public"

# Switch to non-root user
USER formicary-user

# Use the migration script as entrypoint
ENTRYPOINT ["/usr/local/bin/migrate.sh"]
