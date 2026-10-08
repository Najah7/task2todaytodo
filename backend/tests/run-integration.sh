#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

container_name="task2todaytodo-integration-$$"
database_name="task2todaytodo_integration_$$"
postgres_user="integration"
postgres_password="integration"
container_started=0

cleanup() {
	if [ "$container_started" -eq 1 ] && docker container inspect "$container_name" >/dev/null 2>&1; then
		docker stop "$container_name" >/dev/null
	fi
}
trap cleanup EXIT INT TERM

docker run --detach --rm \
	--name "$container_name" \
	-e "POSTGRES_USER=$postgres_user" \
	-e "POSTGRES_PASSWORD=$postgres_password" \
	-e "POSTGRES_DB=$database_name" \
	-p 127.0.0.1::5432 \
	postgres:18.4-trixie >/dev/null
container_started=1

port=""
for _ in $(seq 1 60); do
	port="$(docker port "$container_name" 5432/tcp | sed 's/.*://')"
	if [ -n "$port" ] && docker exec "$container_name" pg_isready -h 127.0.0.1 -U "$postgres_user" -d "$database_name" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
if [ -z "$port" ] || ! docker exec "$container_name" pg_isready -h 127.0.0.1 -U "$postgres_user" -d "$database_name" >/dev/null 2>&1; then
	echo "PostgreSQL integration container did not become ready" >&2
	exit 1
fi

for migration in db/migrations/*.up.sql; do
	docker exec -i "$container_name" psql -q -v ON_ERROR_STOP=1 -U "$postgres_user" -d "$database_name" < "$migration"
done

GOTOOLCHAIN=auto \
INTEGRATION_DATABASE_URL="postgres://$postgres_user:$postgres_password@127.0.0.1:$port/$database_name?sslmode=disable" \
	go test -tags=integration -p 1 ./tests/integration/... -count=1
