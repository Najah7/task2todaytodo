include .env
export

# .SILENT
# .ONESHELL
# .NOTPARALLEL
# .DEFAULT_GOAL := target-name

export PROJECT_ROOT := $(CURDIR)
export MIGRATE_DIR := db/migrations
export MIGRATE_POSTGRES_HOST ?= db
export LOGS_DIR := $(PROJECT_ROOT)/out/logs
export APP_SERVICE_NAME := task2todaytodo


###############################################
# App
###############################################
.PHONY: go-fmt
go-fmt:
	@go fmt ./...

.PHONY: test
test:
	@go test -v ./...

.PHONY: build
build:
	go build -o out/bin/$(APP_SERVICE_NAME) ./cmd/api

.PHONY: run
run:
	@docker compose up --build $(APP_SERVICE_NAME)

.PHONY: env-up
env-up:
	@docker compose up -d db

.PHONY: env-down
env-down:
	@docker compose down db

.PHONY: env-cleanup
env-cleanup:
	@read -p "Would you like to cleanup env? DANGER ACTION! [y/n]: " answer; \
	if [ "$$answer" = "y" ]; then \
		docker volume rm $(APP_SERVICE_NAME)_pgdata; \
	fi;

.PHONY: cleanup-logs
cleanup-logs:
	@read -p "Would you like to cleanup all logs? DANGER OPERATION! [y/n]: " answer; \
	if [ "$$answer" = "y" ]; then \
		rm -rf $(LOGS_DIR); \
	fi;

###############################################
# DB
###############################################

.PHONY: db-shell
db-shell:
	@docker compose exec db psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

.PHONY: migrate-up
migrate-up:
	@$(MAKE) _migrate-action action=up

.PHONY: migrate-down
migrate-down:
	@$(MAKE) _migrate-action action=down

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Name of the migration must be specified! E.g. name=create_user"; \
		exit 1; \
	fi; \
	$(MAKE) _migrate-action action=create name=$(name)

.PHONY: _migrate-action
MIGRATE_CMD = docker compose run --rm migrate
MIGRATE_DATABASE_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(MIGRATE_POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)
_migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Action Must be specified! E.g. action=up"; \
		exit 1; \
	fi; \
	if [ "$(action)" = "create" ]; then \
		if [ -z "$(name)" ]; then \
			echo "Name of the migration must be specified! E.g. name=create_user"; \
			exit 1; \
		fi; \
		$(MIGRATE_CMD) $(action) \
			-dir $(MIGRATE_DIR) \
			-ext sql \
			-seq \
			$(name); \
	else \
		$(MIGRATE_CMD) \
			-path=$(MIGRATE_DIR) \
			-database='$(MIGRATE_DATABASE_URL)' \
			$(action); \
	fi;

.PHONY: sqlc-gen
sqlc-gen:
	rm -rf internal/repositories/sqlc
	@docker compose run --rm --no-deps sqlc generate

###############################################
# Documentation
###############################################

.PHONY: swagger-gen
SWAGGER_CMD = $(MAKE) swagger-action action=init args="-g cmd/api/main.go -o docs --parseInternal"
SWAGGER_CMD_WITH_DEPENDENCY = $(MAKE) swagger-action action=init args="-g cmd/api/main.go -o docs --parseInternal --parseDependency"

swagger-gen: swagger-fmt
	@$(SWAGGER_CMD) || $(SWAGGER_CMD_WITH_DEPENDENCY)

.PHONY: swagger-fmt
swagger-fmt:
	@$(MAKE) swagger-action action=fmt

.PHONY: swagger-action
swagger-action:
	@if [ -z "$(action)" ]; then \
		echo "The parameter action must be provided. E.g. action=init"; \
		exit 1; \
	fi; \
	docker compose run --rm swagger $(action) $(args)
