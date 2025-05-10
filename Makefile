GO := go

GO_MODULE := "github.com/dukunuu/hackathon_backend"
DOCKER_COMPOSE_FILE := docker-compose.yml
GO_APP_SERVICE_NAME := go_app
GO_APP_PROD := go-app

DOCKER_EXEC := docker compose exec -T $(GO_APP_SERVICE_NAME)
DOCKER_EXEC_PROD := docker compose -f docker-compose.prod.yml exec -T $(GO_APP_PROD)
MIGRATIONS_DIR_CONTAINER := ./db/schema/

.DEFAULT_GOAL := help

fmt:
	@echo "▶ Formatting Go code..."
	@$(GO) fmt ./backend/...
	@echo "✔ Formatting complete."

migrate-create:
ifndef NAME
	$(error Usage: make migrate-create NAME=<migration_description>)
endif
	@echo "==> Creating new migration: $(NAME)"
	@mkdir -p backend/$(MIGRATIONS_DIR_CONTAINER)
	$(DOCKER_EXEC) sh -c "migrate create -ext sql -dir $(MIGRATIONS_DIR_CONTAINER) -seq '$(NAME)'"
	@echo "==> Migration file for '$(NAME)' created in $(MIGRATIONS_DIR_CONTAINER) (host: backend/$(MIGRATIONS_DIR_CONTAINER))."

migrate-up:
	@echo "==> Applying database migrations (UP)..."
	@if [ -z "$(VERSION)" ]; then \
		echo "Applying all pending UP migrations..."; \
		$(DOCKER_EXEC) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" up'; \
	else \
		echo "Applying UP migrations to version $(VERSION)..."; \
		$(DOCKER_EXEC) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" up $(VERSION)'; \
	fi
	@echo "==> Migrations UP complete."

migrate-down:
	@echo "==> Rolling back database migrations (DOWN)..."
	@if [ -z "$(VERSION)" ]; then \
		echo "Rolling back the last applied migration (1 step)..."; \
		$(DOCKER_EXEC) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" down 1'; \
	else \
		echo "Rolling back $(VERSION) migration(s)..."; \
		$(DOCKER_EXEC) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" down $(VERSION)'; \
	fi
	@echo "==> Migrations DOWN complete."

migrate-up-prod:
	@echo "==> Applying database migrations (UP)..."
	@if [ -z "$(VERSION)" ]; then \
		echo "Applying all pending UP migrations..."; \
		$(DOCKER_EXEC_PROD) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" up'; \
	else \
		echo "Applying UP migrations to version $(VERSION)..."; \
		$(DOCKER_EXEC_PROD) sh -c 'migrate -path $(MIGRATIONS_DIR_CONTAINER) -database "postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@postgres_db:5432/$${POSTGRES_DB}?sslmode=disable" up $(VERSION)'; \
	fi
	@echo "==> Migrations UP complete."

.PHONY: migrate-create migrate-up migrate-down help

help:
	@echo "Makefile for Database Migrations"
	@echo ""
	@echo "Usage: make [target] [OPTIONS]"
	@echo ""
	@echo "Prerequisites:"
	@echo "  - Docker and Docker Compose installed."
	@echo "  - The 'postgres_db' service should be running (e.g., via 'docker-compose up -d postgres_db')."
	@echo "  - The migration tool (assumed 'migrate') must be installed in the '$(GO_APP_SERVICE_NAME)' container."
	@echo "  - A './backend/.env' file with POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB variables."
	@echo "  - A './backend/migrations' directory for storing migration files (will be created if not present by migrate-create)."
	@echo ""
	@echo "Targets:"
	@echo "  migrate-create NAME=<name>   Create a new SQL migration file."
	@echo "                               Example: make migrate-create NAME=create_users_table"
	@echo "  migrate-up [VERSION=N]       Apply database migrations."
	@echo "                               - Without VERSION: applies all pending up migrations."
	@echo "                               - With VERSION=N: applies migrations up to and including version N."
	@echo "                               Example: make migrate-up"
	@echo "                               Example: make migrate-up VERSION=2"
	@echo "  migrate-down [VERSION=N]     Rollback database migrations."
	@echo "                               - Without VERSION: rolls back the last applied migration (1 step)."
	@echo "                               - With VERSION=N: rolls back N migrations from the current state."
	@echo "                               Example: make migrate-down"
	@echo "                               Example: make migrate-down VERSION=2"
	@echo "  help                         Show this help message."
	@echo ""
	@echo "Notes:"
	@echo "  - Place this Makefile in the same directory as your 'docker-compose.yml'."
	@echo "  - The migration commands use POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB"
	@echo "    environment variables from the '$(GO_APP_SERVICE_NAME)' container's environment,"
	@echo "    which are sourced from its 'env_file' (./backend/.env)."
	@echo "  - If your migration CLI tool is named differently (e.g., 'go-migrate'),"
	@echo "    update the 'migrate' command in the rules above accordingly."


