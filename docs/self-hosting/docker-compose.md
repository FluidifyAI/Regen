# Docker Compose Deployment

The recommended way to run Fluidify Regen in production is with Docker Compose. A single command builds and starts everything.

## Quick start

```bash
git clone https://github.com/fluidifyai/regen.git
cd regen
cp .env.example .env
# Edit .env with your values
make start
```

Open http://your-server-ip:8080

## What runs

| Container | Image | Purpose |
|-----------|-------|---------|
| `fluidify-regen` | Built from repo | Go binary with embedded React frontend |
| `fluidify-regen-db` | `postgres:15-alpine` | Primary datastore |
| `fluidify-regen-redis` | `redis:7-alpine` | Queue and cache |

The app container starts only after PostgreSQL and Redis pass their health checks. Migrations run automatically at startup.

## Commands

| Command | Description |
|---------|-------------|
| `make start` | Build image and start all services (detached) |
| `make stop` | Stop all services |
| `make logs` | Tail logs from all containers |
| `make health` | Check `/health` and `/ready` |
| `make down` | Stop and remove containers (data volumes preserved) |
| `make clean` | Remove containers, volumes, and build artifacts |

## Updating

```bash
git pull
make start   # rebuilds the image automatically
```

Migrations run on startup — no separate migration step needed.

## Persisting data

Data is stored in named Docker volumes:

| Volume | Contents |
|--------|----------|
| `postgres_data` | All application data |
| `redis_data` | Queue state and cache |

Volumes persist across `make stop` / `make start` cycles. Only `make clean` removes them.

**Backup PostgreSQL:**

```bash
docker exec fluidify-regen-db pg_dump -U regen regen > backup-$(date +%Y%m%d).sql
```

**Restore:**

```bash
docker exec -i fluidify-regen-db psql -U regen regen < backup-20240115.sql
```

## Running behind a reverse proxy

For production, put Regen behind nginx or Caddy for TLS termination.

### Caddy (recommended — automatic HTTPS)

```
incidents.yourcompany.com {
    reverse_proxy localhost:8080
}
```

### nginx

```nginx
server {
    listen 443 ssl;
    server_name incidents.yourcompany.com;

    ssl_certificate     /etc/ssl/certs/yourcompany.crt;
    ssl_certificate_key /etc/ssl/private/yourcompany.key;

    location / {
        proxy_pass         http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection 'upgrade';
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

The `X-Forwarded-Proto: https` header is required for Slack OAuth and SAML SSO to construct the correct redirect URIs.

## Custom port

```env
PORT=9000
```

Update your reverse proxy accordingly.

## Environment variables

See [Environment Variables](./environment-variables.md) for the full reference.
