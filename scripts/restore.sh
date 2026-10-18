#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/restore.sh /data/backups/leanprice.sqlite" >&2
  exit 2
fi

backup_path=$1
case "$backup_path" in
  /data/*) ;;
  *) echo "backup path must be inside the leanPrice /data volume" >&2; exit 2 ;;
esac

docker compose stop leanprice
restore_output=$(docker compose run --rm --no-deps leanprice restore "$backup_path") || {
  echo "$restore_output" >&2
  echo "Restore validation failed; the existing database was left in place." >&2
  exit 1
}
printf '%s\n' "$restore_output"
safety_path=$(printf '%s\n' "$restore_output" | sed -n 's/^Restore complete\. Safety backup: //p' | tail -n 1)

docker compose up -d leanprice
healthy=0
attempt=0
while [ "$attempt" -lt 30 ]; do
  if docker compose exec -T leanprice /app/leanprice healthcheck >/dev/null 2>&1; then
    healthy=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done

if [ "$healthy" -eq 1 ]; then
  echo "Restored database passed the application readiness check."
  exit 0
fi

echo "Restored database failed readiness; stopping and reinstating the pre-restore safety snapshot." >&2
docker compose stop leanprice || true
if [ -n "$safety_path" ]; then
  docker compose run --rm --no-deps leanprice restore "$safety_path"
  docker compose up -d leanprice
else
  echo "No pre-restore database existed, so there is no safety snapshot to reinstate." >&2
fi
exit 1
