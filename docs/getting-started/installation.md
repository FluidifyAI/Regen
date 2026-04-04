# Installation

Fluidify Regen is a self-hosted application. It runs as a single Docker container alongside PostgreSQL and Redis. No cloud account required.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- A server or local machine with at least 1 GB RAM
- Ports 8080 (or your chosen `PORT`) accessible

## Quick start

```bash
git clone https://github.com/fluidifyai/regen.git
cd regen
cp .env.example .env   # edit with your values
make start
```

Open **http://localhost:8080** — the app is running.

On first load you will be prompted to create your admin account.

## What `make start` does

`make start` runs `docker-compose up --build -d`. It:

1. Builds the production Docker image (React frontend embedded in the Go binary)
2. Starts PostgreSQL and Redis
3. Runs all database migrations automatically
4. Seeds the AI agents and sample data
5. Starts the application on port 8080

Everything is in a single binary — no separate frontend container, no NGINX, no reverse proxy needed to get started.

## Useful commands

| Command | Description |
|---------|-------------|
| `make start` | Build and start everything |
| `make stop` | Stop all containers |
| `make logs` | Tail logs |
| `make health` | Check `/health` and `/ready` endpoints |

## Verifying the install

```bash
curl http://localhost:8080/health
# {"status":"ok"}

curl http://localhost:8080/ready
# {"status":"ready","database":"ok","redis":"ok"}
```

## Next steps

- [Configure environment variables](../self-hosting/environment-variables.md) — required before connecting integrations
- [Connect Slack](./connecting-slack.md) — receive alerts and manage incidents from Slack
- [Connect a monitoring source](../alerts/sources/prometheus.md) — start sending alerts
