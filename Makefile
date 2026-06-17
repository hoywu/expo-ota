SHELL := bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.ONESHELL:
.PHONY: gen-admin-api gen-protocol-api new-sql migrate gen-model dev-admin dev-protocol asset-gc test infra-up infra-down infra-clear infra-log

SERVER_ENV_FILE := server/deploy/.env
define with_server_env
	@( set -a; . $(SERVER_ENV_FILE); set +a; cd server && $(1) )
endef

# Code generation
gen-admin-api:
	$(call with_server_env, goctl api go --api ./api/admin/admin.api --dir ./api/admin)

gen-protocol-api:
	$(call with_server_env, goctl api go --api ./api/protocol/protocol.api --dir ./api/protocol)

# Database migrations
new-sql:
	@read -p "Enter migration name: " name
	cd server/db/migrations
	goose -s create "$$name" sql

migrate:
	$(call with_server_env, go run ./db)

TABLES := users,apps,code_signing_keys,runtime_versions,assets,updates,update_assets,api_tokens,manifest_requests,client_events,audit_logs
gen-model:
	$(call with_server_env, goctl model pg datasource --url="$$DB_URL" --table="$(TABLES)" --cache=false --dir=./db/models)

# Development
dev-admin:
	$(call with_server_env, go run ./api/admin -f ./api/admin/etc/admin-api.yaml)

dev-protocol:
	$(call with_server_env, go run ./api/protocol -f ./api/protocol/etc/protocol-api.yaml)

# Orphan asset garbage collection (run from cron in production)
asset-gc:
	$(call with_server_env, go run ./cmd/asset-gc)

# Tests
test:
	cd server && go test ./...

# Docker infrastructure (dev)
infra-up:
	docker compose -f server/deploy/docker-compose.infra.yml up -d

infra-down:
	docker compose -f server/deploy/docker-compose.infra.yml down

infra-clear:
	docker compose -f server/deploy/docker-compose.infra.yml down --volumes

infra-log:
	docker compose -f server/deploy/docker-compose.infra.yml logs -f
