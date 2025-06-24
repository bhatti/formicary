#!/bin/bash -e

# Default values (can be overridden by environment variables)
DB_TYPE="${DB_TYPE:-postgres}"
DB_NAME="${DB_NAME:-formicary_db}"
DB_USER="${DB_USER:-formicary_user}"
DB_PASSWORD="${DB_PASSWORD:-formicary_pass}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_SSL_MODE="${DB_SSL_MODE:-disable}"

# Construct connection string based on database type
case "$DB_TYPE" in
    "postgres")
        CONNECTION_STRING="user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME host=$DB_HOST port=$DB_PORT sslmode=$DB_SSL_MODE"
        DB_DATA_SOURCE="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:5432/${DB_NAME}?sslmode=disable"
        ;;
    "mysql")
        CONNECTION_STRING="$DB_USER:$DB_PASSWORD@tcp($DB_HOST:$DB_PORT)/$DB_NAME"
        DB_DATA_SOURCE="${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:3306)/${DB_NAME}?charset=utf8mb4&parseTime=True&loc=Local"
        ;;
    "sqlite")
        CONNECTION_STRING="$DB_NAME"
        mkdir -p /data
        DB_SQLITE_PATH="/data/formicary.db"
        DB_DATA_SOURCE="$DB_SQLITE_PATH"
        ;;
    *)
        echo "Unsupported database type: $DB_TYPE"
        exit 1
        ;;
esac

echo "Running database migrations..."
echo "Database type: $DB_TYPE"
echo "Connection: $DB_HOST:$DB_PORT/$DB_NAME"

# Run migrations
if ! goose -dir /migrations "$DB_TYPE" "$CONNECTION_STRING" up; then
    echo "Migration failed!"
    exit 1
fi

echo "Migrations completed successfully!"

# Execute the main application with all passed arguments
exec /formicary "$@"

