SHELL := bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.ONESHELL:
.PHONY: infra-up infra-down infra-log

# Docker infrastructure (dev)
infra-up:
	docker compose -f server/deploy/docker-compose.infra.yml up -d

infra-down:
	docker compose -f server/deploy/docker-compose.infra.yml down

infra-log:
	docker compose -f server/deploy/docker-compose.infra.yml logs -f
