SHELL := bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory
.ONESHELL:
.PHONY: gen-admin-api gen-protocol-api new-sql migrate gen-model dev dev-admin dev-protocol dev-dashboard asset-gc test infra-up infra-down infra-clear infra-log

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
dev:
	@$(MAKE) -s infra-up
	$(MAKE) -s migrate
	graceful_stop() {
		kill_tree() {
			local sig=$$1 pid=$$2 child
			for child in $$(pgrep -P "$$pid" 2>/dev/null || true); do kill_tree "$$sig" "$$child"; done
			kill -"$$sig" "$$pid" 2>/dev/null || true
		}
		for pid in "$$@"; do kill_tree INT "$$pid"; done
		for pid in "$$@"; do
			for i in $$(seq 1 50); do
				kill -0 "$$pid" 2>/dev/null || break
				if [[ $$i -eq 50 ]]; then kill_tree KILL "$$pid"; fi
				sleep 0.2
			done
			wait "$$pid" 2>/dev/null || true
		done
	}
	admin_fifo=$$(mktemp -u); protocol_fifo=$$(mktemp -u)
	mkfifo "$$admin_fifo" "$$protocol_fifo"
	sed -u 's/^/[admin] /' <"$$admin_fifo" & sed_admin_pid=$$!
	sed -u 's/^/[protocol] /' <"$$protocol_fifo" & sed_protocol_pid=$$!
	$(MAKE) -s dev-admin >"$$admin_fifo" 2>&1 & admin_pid=$$!
	$(MAKE) -s dev-protocol >"$$protocol_fifo" 2>&1 & protocol_pid=$$!
	rm -f "$$admin_fifo" "$$protocol_fifo"
	trap 'graceful_stop $$admin_pid $$protocol_pid $$sed_admin_pid $$sed_protocol_pid; exit 130' INT TERM
	rc=0
	$(MAKE) -s dev-dashboard || rc=$$?
	graceful_stop $$admin_pid $$protocol_pid $$sed_admin_pid $$sed_protocol_pid
	exit $$rc

dev-admin:
	$(call with_server_env, go run ./api/admin -f ./api/admin/etc/admin-api.yaml)

dev-protocol:
	$(call with_server_env, go run ./api/protocol -f ./api/protocol/etc/protocol-api.yaml)

dev-dashboard:
	cd dashboard && bun run dev

# Orphan asset garbage collection (run from cron in production)
asset-gc:
	$(call with_server_env, go run ./cmd/asset-gc)

# Tests
test:
	cd server && go test ./...

# Docker infrastructure (dev)
infra-up:
	docker compose -f server/deploy/docker-compose.infra.yml up -d --wait

infra-down:
	docker compose -f server/deploy/docker-compose.infra.yml down

infra-clear:
	docker compose -f server/deploy/docker-compose.infra.yml down --volumes

infra-log:
	docker compose -f server/deploy/docker-compose.infra.yml logs -f
