#!/bin/sh
set -eu

environment="${1:?environment is required}"
compose_file="${2:?compose file is required}"
health_url="${3:?health URL is required}"
shift 3
[ "$#" -gt 0 ] || { echo "at least one service is required" >&2; exit 2; }

env_file="deploy/$environment.env"
[ -f "$env_file" ] || { echo "missing $env_file; copy the matching .env.example" >&2; exit 2; }

compose="docker compose --env-file $env_file -f $compose_file"
services="$*"

echo "Deploying $services in $environment using $compose_file."
echo "This deploy touches only: $services. PostgreSQL and frontend are not recreated."
$compose pull $services
$compose up -d --no-deps --force-recreate $services
docker image prune -f

timeout_seconds="${DEPLOY_HEALTH_TIMEOUT_SECONDS:-120}"
deadline=$(( $(date +%s) + timeout_seconds ))
until curl --fail --silent --show-error "$health_url" >/dev/null 2>&1; do
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "Health check failed for $health_url" >&2
    $compose ps >&2 || true
    $compose logs --tail=100 $services >&2 || true
    exit 1
  fi
  sleep 2
done

echo "Deployment healthy:"
$compose ps $services
