#!/bin/sh
set -eu
exec "$(dirname "$0")/deploy.sh" production docker-compose.production.yml "${PRODUCTION_HEALTH_URL:-http://localhost:8080/healthz}" backend worker
