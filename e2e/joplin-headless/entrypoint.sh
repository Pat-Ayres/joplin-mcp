#!/bin/sh
set -e

TOKEN="${JOPLIN_API_TOKEN:-test-token}"

# Configure the API token so requests can authenticate.
joplin config api.token "$TOKEN"

# Start the Joplin clipper/data API server.
# It binds to 127.0.0.1:41184 (hard-coded in the CLI).
joplin server start &
JOPLIN_PID=$!

# Wait for the API to become ready.
echo "Waiting for Joplin API on localhost:41184..."
for i in $(seq 1 60); do
    if curl -sf http://127.0.0.1:41184/ping >/dev/null 2>&1; then
        echo "Joplin API is ready (took ${i}s)."
        break
    fi
    if [ "$i" -eq 60 ]; then
        echo "ERROR: Joplin API did not start within 60s" >&2
        exit 1
    fi
    sleep 1
done

# Forward external traffic to the localhost-only API.
echo "Starting socat forwarder on 0.0.0.0:8080 -> 127.0.0.1:41184..."
socat TCP-LISTEN:8080,fork,reuseaddr TCP:127.0.0.1:41184 &

# Keep the container running until the Joplin process exits.
wait $JOPLIN_PID
