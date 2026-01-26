#!/bin/sh
set -e

echo "Waiting for Postgres at $POSTGRES_HOST:5432..."
until nc -z "$POSTGRES_HOST" "5432"; do
  echo "Postgres not ready, sleeping 1s..."
  sleep 1
done
echo "Postgres is up!"

echo "Starting app..."
exec ./rpsync
