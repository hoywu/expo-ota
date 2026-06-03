SHELL := bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.ONESHELL:
.PHONY: new-sql migrate dev-admin dev-protocol infra-up infra-down infra-clear infra-log

SERVER_ENV_FILE := server/deploy/.env
define with_server_env
	@( set -a; . $(SERVER_ENV_FILE); set +a; cd server && $(1) )
endef

# Database migrations
new-sql:
	@read -p "Enter migration name: " name
	cd server/db/migrations
	goose -s create "$$name" sql

migrate:
	$(call with_server_env, go run ./db)

# Development
dev-admin:
	$(call with_server_env, go run ./api/admin -f ./api/admin/etc/admin-api.yaml)

dev-protocol:
	$(call with_server_env, go run ./api/protocol -f ./api/protocol/etc/protocol-api.yaml)

# Docker infrastructure (dev)
infra-up:
	docker compose -f server/deploy/docker-compose.infra.yml up -d

infra-down:
	docker compose -f server/deploy/docker-compose.infra.yml down

infra-clear:
	docker compose -f server/deploy/docker-compose.infra.yml down --volumes

infra-log:
	docker compose -f server/deploy/docker-compose.infra.yml logs -f
