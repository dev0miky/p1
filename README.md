# p1

**Outbound calling platform — press-1 campaigns and voice broadcast, built for US TCPA compliance.**

Self-hosted, multi-tenant, no third-party dialer. Kamailio at the SIP edge, FreeSWITCH for media, a Go engine for pacing and call state, Postgres with row-level security for tenant isolation, and two React frontends. Everything runs in Docker.

[![ci](https://github.com/dev0miky/p1/actions/workflows/ci.yml/badge.svg)](https://github.com/dev0miky/p1/actions/workflows/ci.yml)
[![license: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
![status: pre-release](https://img.shields.io/badge/status-pre--release-orange.svg)

---

## What it does

- **Press-1 campaigns** — dial a list, play a greeting, bridge the people who press a key to a live agent (external SIP destination), capture opt-outs.
- **Voice broadcast** — dial a list, play a message, hang up; detect answering machines and drop a clean voicemail.
- **Multi-tenant by design** — one Postgres database, `tenant_id` on every business row, enforced by Postgres RLS. Tenants self-serve through a console; a super-admin runs the platform.
- **Compliance is first-class** — calling-hours windows, internal DNC scrubbing at originate time, DTMF opt-out → DNC, abandon-rate accounting, 4-year recording retention.

## Architecture

```
        React console  +  React agent app
                     │  HTTPS / WSS (Caddy)
                     ▼
        ┌─────────────────────────────┐
        │  Go engine                  │   pacing · lead claim (SKIP LOCKED)
        │  api · dialer · cdr-ingest  │   per-call FSM · compliance preflight
        │  recording-uploader         │
        └───┬───────────┬────────┬────┘
            │           │        │
       ┌────▼───┐  ┌────▼──┐  ┌──▼──────────┐
       │Postgres│  │ Redis │  │  ESL events │
       │  RLS   │  │pacing │  │ (in-process)│
       └────────┘  └───────┘  └──▲──────────┘
                                 │
                       ┌─────────┴──────────┐
                       │   FreeSWITCH       │  mod_avmd (voicemail beep)
                       │   lua per-call IVR │  record_session → MinIO
                       └─────────┬──────────┘
                                 │ SIP / RTP
                       ┌─────────▼──────────┐
                       │   Kamailio edge    │  dispatcher · pike · secsipid
                       └─────────┬──────────┘
                                 │
                            carrier trunks
```

## Stack

| Layer | Choice |
|---|---|
| SIP edge | Kamailio |
| Media | FreeSWITCH 1.10 (`mod_avmd`, `mod_lua`) |
| Engine | Go (pgx, chi, eslgo) |
| State | PostgreSQL 16 + row-level security |
| Hot state | Redis 7 (pacing, presence) |
| Storage | MinIO (recordings, 4-year retention) |
| Frontends | React + Vite + TanStack Query/Router + Tailwind |
| Edge / TLS | Caddy (automatic Let's Encrypt) |
| Observability | Prometheus, Grafana, Loki, HOMER |
| Runtime | Docker Compose (dev) → Docker Swarm (prod) |

## Quick start (dev)

Requires Docker 24+ and `make`.

```sh
cp .env.example .env
make up
make seed      # dev super_admin + sample tenant
make logs
```

| Service | URL |
|---|---|
| Console (tenant + admin) | http://localhost:3001 |
| Agent app | http://localhost:3002 |
| API | http://localhost:8080 |
| MinIO console | http://localhost:9001 |
| Grafana | http://localhost:3000 |
| HOMER (SIP capture) | http://localhost:9080 |

## Layout

```
deploy/   docker images, FreeSWITCH/Kamailio/Caddy configs, swarm stacks
engine/   go services — api, dialer, cdr-ingest, recording-uploader, dnc-sync
apps/     react frontends — console (tenant + admin), agent
ops/      runbooks, grafana dashboards, helper scripts
docs/     product + compliance documentation
```

## Make targets

```sh
make up              # bring everything up (dev profile)
make down            # stop + remove containers
make logs            # tail all logs (use s=<service> for one)
make ps              # service status
make psql            # psql shell
make fs-cli          # freeswitch console
make kam-cli         # kamailio cli
make migrate         # run engine migrations
make seed            # seed dev super_admin + sample tenant
make test            # run the full test suite
```

## Compliance

Before placing real calls, operators are responsible for: prior express written consent records, federal + state DNC scrubbing, calling-hour windows in the called party's local time, STIR/SHAKEN attestation on owned DIDs, an abandon-rate safe harbor, and recording-disclosure where required. The platform provides the enforcement points; the legal posture is yours.

## Production

Production runs on bare metal (never shared-NIC cloud for media — RTP jitter). Swarm stack files live in `deploy/swarm/`, operational runbooks in `ops/runbooks/`.

## License

[GNU AGPL-3.0](LICENSE). If you run a modified version as a network service, you must make your source available. For commercial terms without the copyleft obligation, contact the maintainer.
