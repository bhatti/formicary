FROM golang:1.22.0-alpine as go-builder
COPY . /src
WORKDIR /src

RUN apk add --no-cache git make bash build-base sqlite sqlite-dev && \
    go mod download && make build-linux


# Install Goose in the builder stage
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
#apk del build-base

FROM alpine:latest

# Install runtime dependencies - postgresql-client
RUN apk add --no-cache ca-certificates sqlite bash && \
    apk add --no-cache mysql-client && \
    addgroup -S formicary-user && \
    adduser -S -G formicary-user formicary-user 

# Copy the built binary from the build stage
COPY --from=go-builder /src/out/bin/formicary /formicary

# Copy the Goose binary from the go-builder stage
COPY --from=go-builder /go/bin/goose /usr/local/bin

# Copy the public directory from the build stage
COPY --from=go-builder /src/public /public

# Copy the migrations directory
COPY --from=go-builder /src/migrations /migrations

# Make sure go and goose are in the PATH for the formicary-user
ENV PATH="/home/formicary-user/go/bin:${PATH}"

# Initialize environment variables with default values if not already set
ENV DB_NAME="${DB_NAME:-formicary_db}" \
    DB_USER="${DB_USER:-formicary_user}" \
    DB_PASSWORD="${DB_PASSWORD:-formicary_pass}" \
    DB_HOST="${DB_HOST:-localhost}" \
    DB_PORT="${DB_PORT:-3306}" \
    DB_ROOT_USER="${DB_ROOT_USER:-root}" \
    DB_TYPE="${DB_TYPE:-postgres}" \
    DB_SSL_MODE="${DB_SSL_MODE:-disable}" \
    DB_ROOT_PASSWORD="${DB_ROOT_PASSWORD:-rootroot}" \
    PUBLIC_DIR="${PUBLIC_DIR:-/public}" 

# Switch to the non-root user for security
USER formicary-user

# The ENTRYPOINT or CMD should be updated to run the migrations
ENTRYPOINT ["sh", "-c", "goose -dir /migrations $DB_TYPE \"user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME host=$DB_HOST port=$DB_PORT sslmode=$DB_SSL_MODE\" up && exec /formicary \"$@\"", "--"]
