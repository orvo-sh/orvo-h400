CH_MIGRATIONS_DIR := internal/infra/clickhouse-db/migrations
PG_MIGRATIONS_DIR := internal/infra/postgres-db/migrations
CLIENT_OUTPUT_DIR := frontend/src/lib/api

dev-ingest:
	wgo run ./cmd/ingest

dev-app:
	wgo run ./cmd/orvo
	
create-ch-migration:
ifndef NAME
	$(error NAME is required. Usage: make create-ch-migration NAME=<migration_name>)
endif
	@mkdir -p $(CH_MIGRATIONS_DIR)
	@goose -dir $(CH_MIGRATIONS_DIR) create $(NAME) sql
	@echo "Migration file created in $(CH_MIGRATIONS_DIR) with name: $(NAME)"

create-pg-migration:
ifndef NAME
	$(error NAME is required. Usage: make create-pg-migration NAME=<migration_name>)
endif
	@mkdir -p $(PG_MIGRATIONS_DIR)
	@goose -dir $(PG_MIGRATIONS_DIR) create $(NAME) sql
	@echo "Migration file created in $(PG_MIGRATIONS_DIR) with name: $(NAME)"

# Run PostgreSQL migrations
migrate-pg:
	@goose -dir $(PG_MIGRATIONS_DIR) postgres "$$DATABASE_URL" up

migrate-pg-down:
	@goose -dir $(PG_MIGRATIONS_DIR) postgres "$$DATABASE_URL" down

# Generate SQLC code
sqlc:
	sqlc generate

# Generate TypeScript API client from OpenAPI spec
# Option 1: Using openapi-typescript-codegen (recommended for SvelteKit)
generate-client:
	@echo "Generating TypeScript API client..."
	@mkdir -p $(CLIENT_OUTPUT_DIR)
	npx openapi-typescript-codegen \
		--input http://localhost:3000/api/openapi.json \
		--output $(CLIENT_OUTPUT_DIR) \
		--client fetch \
		--useOptions
	@echo "Client generated in $(CLIENT_OUTPUT_DIR)"

# Option 2: Using openapi-fetch (lighter weight, better for fetch-based clients)
generate-client-types:
	@echo "Generating TypeScript types from OpenAPI spec..."
	@mkdir -p $(CLIENT_OUTPUT_DIR)
	npx openapi-typescript http://localhost:3000/api/openapi.json -o $(CLIENT_OUTPUT_DIR)/schema.d.ts
	@echo "Types generated in $(CLIENT_OUTPUT_DIR)/schema.d.ts"

# Option 3: Manual client generation with custom template
# This creates a simple typed fetch wrapper
generate-client-manual:
	@echo "Generating manual TypeScript client..."
	@mkdir -p $(CLIENT_OUTPUT_DIR)
	@go run ./cmd/generate-client --output $(CLIENT_OUTPUT_DIR)/client.ts
	@echo "Client generated in $(CLIENT_OUTPUT_DIR)/client.ts"

# Generate OpenAPI spec from handlers
# Uses swaggo/swag or similar to extract OpenAPI from Go code
generate-openapi:
	@echo "Generating OpenAPI specification..."
	swag init -g cmd/orvo/main.go -o ./docs --parseDependency --parseInternal
	@echo "OpenAPI spec generated in ./docs"

# All-in-one: generate OpenAPI spec and then generate client
generate-all: generate-openapi generate-client

# Clean generated files
clean-generated:
	rm -rf $(CLIENT_OUTPUT_DIR)
	rm -rf ./docs/swagger.*

.PHONY: dev-ingest dev-app create-ch-migration create-pg-migration migrate-pg migrate-pg-down sqlc generate-client generate-client-types generate-client-manual generate-openapi generate-all clean-generated
