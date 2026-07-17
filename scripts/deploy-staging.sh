#!/bin/sh
set -eu
exec "$(dirname "$0")/deploy.sh" staging docker-compose.staging.yml "${STAGING_HEALTH_URL:-http://localhost:18080/healthz}" backend worker
