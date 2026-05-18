# p1

Outbound calling platform — press-1 campaigns and voice broadcast.

Multi-tenant, open-source stack: Kamailio + FreeSWITCH + Go + Postgres + React.

## Quick start (dev)

Requires Docker 24+ and `make`.

```sh
cp .env.example .env
make up
make logs
```

Services come up at:

| Service | URL |
|---|---|
| Console (tenant + admin) | http://localhost:3001 |
| Agent app | http://localhost:3002 |
| API | http://localhost:8080 |
| Postgres | localhost:5432 (user `p1`) |
| Redis | localhost:6379 |
| MinIO console | http://localhost:9001 |
| Grafana | http://localhost:3000 |
| HOMER (SIP capture) | http://localhost:9080 |

## Layout

```
deploy/   docker images, configs, swarm stacks
engine/   go services (dialer, api, cdr-ingest, recording-uploader, dnc-sync)
apps/     react frontends (console, agent, shared ui-kit)
ops/      runbooks, dashboards, helper scripts
docs/     product + compliance documentation
```

## Common make targets

```sh
make up              # bring everything up (dev profile)
make down            # stop + remove containers
make logs            # tail all logs
make logs s=engine   # tail one service
make ps              # service status
make psql            # psql shell to postgres
make redis-cli       # redis shell
make fs-cli          # freeswitch console
make kam-cli         # kamailio cli
make migrate         # run engine migrations
make seed            # seed dev super_admin + sample tenant
make test            # run all tests
```

## Production deploy

See `deploy/swarm/` and `ops/runbooks/`.
