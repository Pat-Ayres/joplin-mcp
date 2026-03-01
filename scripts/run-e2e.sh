#!/usr/bin/env bash
# Run the e2e test suite via Docker Compose.
#
# This builds the headless Joplin container and the Go test runner,
# waits for Joplin to be healthy, then runs the test suite.
#
# Usage: ./scripts/run-e2e.sh
set -euo pipefail

COMPOSE_FILE="docker-compose.test.yml"

echo "==> Building and starting e2e stack..."
docker compose -f "$COMPOSE_FILE" up \
    --build \
    --abort-on-container-exit \
    --exit-code-from e2e
EXIT_CODE=$?

echo "==> Tearing down e2e stack..."
docker compose -f "$COMPOSE_FILE" down -v

exit $EXIT_CODE
