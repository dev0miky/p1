SHELL := /bin/bash
COMPOSE := docker compose -f docker-compose.yml -f docker-compose.dev.yml
COMPOSE_PROD := docker compose -f docker-compose.yml -f docker-compose.prod.yml

.PHONY: help up down restart logs ps build pull psql redis-cli fs-cli kam-cli minio-cli migrate seed test lint fmt clean nuke prod-up prod-down

help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

up: ## bring full dev stack up
	$(COMPOSE) up -d
	@echo
	@$(COMPOSE) ps

down: ## stop and remove dev stack
	$(COMPOSE) down

restart: ## restart all services
	$(COMPOSE) restart

logs: ## tail logs (use s=<service> for one)
	@if [ -z "$(s)" ]; then $(COMPOSE) logs -f --tail=100; else $(COMPOSE) logs -f --tail=200 $(s); fi

ps: ## service status
	$(COMPOSE) ps

build: ## rebuild local images
	$(COMPOSE) build

pull: ## pull upstream images
	$(COMPOSE) pull

psql: ## psql shell
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-p1} -d $${POSTGRES_DB:-p1}

redis-cli: ## redis shell
	$(COMPOSE) exec redis redis-cli

fs-cli: ## freeswitch cli
	$(COMPOSE) exec freeswitch fs_cli

kam-cli: ## kamailio cli
	$(COMPOSE) exec kamailio kamcmd

minio-cli: ## mc against local minio
	$(COMPOSE) exec minio mc alias set local http://localhost:9000 $${MINIO_ROOT_USER:-p1} $${MINIO_ROOT_PASSWORD:-p1minio_change_me} && $(COMPOSE) exec minio mc ls local

migrate: ## run engine migrations
	$(COMPOSE) run --rm engine-api /app/api migrate

seed: ## seed super_admin + sample tenant
	$(COMPOSE) run --rm engine-api /app/api seed

test: ## run all tests
	cd engine && go test -count=1 -p 1 ./...
	cd apps/console && npm test --silent
	cd apps/agent && npm test --silent

lint:
	cd engine && golangci-lint run
	cd apps/console && npm run lint
	cd apps/agent && npm run lint

fmt:
	cd engine && gofmt -s -w .
	cd apps/console && npm run format
	cd apps/agent && npm run format

clean: ## remove containers, keep volumes
	$(COMPOSE) down --remove-orphans

nuke: ## remove everything including volumes (destroys data)
	$(COMPOSE) down -v --remove-orphans

prod-up:
	$(COMPOSE_PROD) up -d

prod-down:
	$(COMPOSE_PROD) down
