#!/bin/sh
# Идемпотентно создаёт роль и БД приложения на кластере Spilo/Patroni (суперпользователь postgres).
# Подстановка :'name' в psql работает только вне dollar-quote; используем SELECT ... \gexec.
set -eu
PGHOST="${PGHOST:-spilo}"
PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-postgres}"
export PGPASSWORD="${PGPASSWORD:?missing PGPASSWORD}"

until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" >/dev/null 2>&1; do
	sleep 2
done

psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 \
	-v app_user="${APP_DB_USER}" \
	-v app_pass="${APP_DB_PASSWORD}" \
	-v app_db="${APP_DB_NAME}" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_user', :'app_pass')
WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = :'app_user')\gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'app_db', :'app_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'app_db')\gexec
SQL
