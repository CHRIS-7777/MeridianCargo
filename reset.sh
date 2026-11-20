#!/usr/bin/env bash
# Reset CHRIS-LAB to its pristine state (database re-seeded, uploads and logs discarded).
set -e
cd "$(dirname "$0")"
docker compose down -v
docker compose up -d --build
echo "Waiting for the portal..."
for i in $(seq 1 60); do
  if curl -fs -o /dev/null http://localhost:8090/; then echo "Ready: http://localhost:8090/"; exit 0; fi
  sleep 2
done
echo "Portal did not come up in time; check: docker compose logs" >&2
exit 1
