#!/bin/sh
set -e

echo "Running database migrations..."
MAX_RETRIES=30
RETRY_COUNT=0

until goose -dir ./migrations postgres "postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}" up; do
  RETRY_COUNT=$((RETRY_COUNT + 1))
  if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
    echo "Failed to run migrations after $MAX_RETRIES attempts"
    exit 1
  fi
  echo "Migration failed, retrying in 2 seconds... (attempt $RETRY_COUNT/$MAX_RETRIES)"
  sleep 2
done

echo "Migrations complete. Starting application..."
exec /app/pr-service
