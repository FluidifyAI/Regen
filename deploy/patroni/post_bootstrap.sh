#!/bin/sh
set -e
psql -U postgres -c "CREATE USER regen WITH PASSWORD 'secret' CREATEROLE CREATEDB;"
psql -U postgres -c "CREATE DATABASE regen OWNER regen;"
