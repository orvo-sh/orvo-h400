PG_MIGRATIONS_DIR := internal/infra/postgres/migrations

# Development
dev:
	wgo run ./cmd/orvo

dev-ingest:
	wgo run ./cmd/ingest

# Database migrations
create-migration:
ifndef NAME
	$(error NAME is required. Usage: make create-migration NAME=<migration_name>)
endif
	@mkdir -p $(PG_MIGRATIONS_DIR)
	@goose -dir $(PG_MIGRATIONS_DIR) create $(NAME) sql
	@echo "Migration created: $(PG_MIGRATIONS_DIR)/$(NAME)"

migrate:
	@goose -dir $(PG_MIGRATIONS_DIR) postgres "$$DATABASE_URL" up

migrate-down:
	@goose -dir $(PG_MIGRATIONS_DIR) postgres "$$DATABASE_URL" down


gen-openapi:
	go run ./cmd/openapi > openapi.yaml


sqlc:
	sqlc generate

# Generate TypeScript API client from OpenAPI spec
# Requires the server to be running on localhost:8080
generate-api:
	@echo "Generating TypeScript API client from OpenAPI spec..."
	@echo "Make sure the server is running: make dev"
	cd frontend && pnpm run generate-api
	@echo "API client generated: frontend/src/lib/api/schema.ts"

.PHONY: dev dev-ingest create-migration migrate migrate-down sqlc generate-api
