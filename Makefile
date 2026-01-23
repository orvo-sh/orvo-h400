CH_MIGRATIONS_DIR := internal/infra/clickhouse-db/migrations

create-ch-migration:
ifndef NAME
	$(error NAME is required. Usage: make create-ch-migration NAME=<migration_name>)
endif
	@mkdir -p $(CH_MIGRATIONS_DIR)
	@goose -dir $(CH_MIGRATIONS_DIR) create $(NAME) sql
	@echo "Migration file created in $(CH_MIGRATIONS_DIR) with name: $(NAME)"
