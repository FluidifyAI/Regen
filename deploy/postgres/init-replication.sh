#!/bin/bash
# init-replication.sh
#
# Runs once on first startup of pg-primary via docker-entrypoint-initdb.d.
# Creates a replication user and configures pg_hba.conf to allow the
# replica to connect for streaming replication.

set -e

PG_USER="${POSTGRES_USER:-regen}"

psql -v ON_ERROR_STOP=1 --username "$PG_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create a dedicated replication user so the replica connects with
    -- least-privilege credentials (not the main app user).
    DO \$\$
    BEGIN
      IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'replicator') THEN
        CREATE ROLE replicator WITH LOGIN REPLICATION PASSWORD 'replicator_secret';
      END IF;
    END
    \$\$;
EOSQL

# Allow replication connections from any host on the Docker network.
# In production this should be restricted to the replica's IP/subnet.
echo "host replication replicator 0.0.0.0/0 md5" >> "$PGDATA/pg_hba.conf"

echo "Replication user and pg_hba rule created."
