#!/bin/bash
set -e
echo "Starting Formicary migration and startup..."
# Default values (can be overridden by environment variables)
DB_TYPE="${DB_TYPE:-sqlite}"
DB_NAME="${DB_NAME:-formicary_db}"
DB_USER="${DB_USER:-formicary_user}"
DB_PASSWORD="${DB_PASSWORD:-formicary_pass}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_SSL_MODE="${DB_SSL_MODE:-disable}"
echo "Database type: $DB_TYPE"
# Construct connection string based on database type
case "$DB_TYPE" in
    "postgres")
        CONNECTION_STRING="user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME host=$DB_HOST port=$DB_PORT sslmode=$DB_SSL_MODE"
        DB_DATA_SOURCE="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:5432/${DB_NAME}?sslmode=disable"
        echo "Connection: $DB_HOST:$DB_PORT/$DB_NAME"
        ;;
    "mysql")
        CONNECTION_STRING="$DB_USER:$DB_PASSWORD@tcp($DB_HOST:$DB_PORT)/$DB_NAME"
        DB_DATA_SOURCE="${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:3306)/${DB_NAME}?charset=utf8mb4&parseTime=True&loc=Local"
        echo "Connection: $DB_HOST:$DB_PORT/$DB_NAME"
        ;;
    "sqlite")
        # DB_DATA_SOURCE may be set via env (from ConfigMap/k8s); default to /data/db/formicary.db
        DB_SQLITE_PATH="${DB_DATA_SOURCE:-/data/db/formicary.db}"
        CONNECTION_STRING="$DB_SQLITE_PATH"
        DB_DATA_SOURCE="$DB_SQLITE_PATH"
        echo "SQLite database: $DB_SQLITE_PATH"
        DB_SQLITE_DIR=$(dirname "$DB_SQLITE_PATH")
        mkdir -p "$DB_SQLITE_DIR"
        if [ ! -w "$DB_SQLITE_DIR" ]; then
            echo "ERROR: $DB_SQLITE_DIR is not writable"
            ls -la "$DB_SQLITE_DIR" 2>/dev/null
            exit 1
        fi
        echo "$DB_SQLITE_DIR is writable ✓"
        ;;
    *)
        echo "Unsupported database type: $DB_TYPE"
        exit 1
        ;;
esac
# Check if goose is available
echo "Checking goose availability..."
if ! command -v goose >/dev/null 2>&1; then
    echo "ERROR: goose command not found"
    echo "Available commands in /usr/local/bin:"
    ls -la /usr/local/bin/
    echo "PATH: $PATH"
    exit 1
fi
# Test goose version
echo "Goose version:"
goose --version || {
    echo "ERROR: goose command failed"
    exit 1
}
echo "Running database migrations..."
# Run migrations
if [ -d "/migrations" ]; then
    echo "Found migrations directory, contents:"
    ls -la /migrations/
    # Use the correct goose command format
    if ! goose -dir /migrations "$DB_TYPE" "$CONNECTION_STRING" up; then
        echo "Migration failed!"
        echo "Goose command was: goose -dir /migrations $DB_TYPE $CONNECTION_STRING up"
        exit 1
    fi
    echo "Migrations completed successfully!"
else
    echo "WARNING: No migrations directory found at /migrations"
fi
