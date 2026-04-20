APP_NAME := bugrail
CMD_PKG := ./cmd/bugrail
DATA_DIR := ./.data
DEV_DB := $(DATA_DIR)/bugrail.sqlite3
DOCKER_IMAGE ?= bugrail:dev
DOCKER_PLATFORMS ?= linux/amd64
GORELEASER_CONFIG := goreleaser.yml

.PHONY: dev test test-integration lint generate migrate build docker db-reset db-shell

dev:
	@mkdir -p $(DATA_DIR)
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found; running go run $(CMD_PKG) serve"; \
		go run $(CMD_PKG) serve; \
	fi

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

generate:
	sqlc generate

migrate:
	@mkdir -p $(DATA_DIR)
	go run $(CMD_PKG) migrate

build:
	goreleaser build --snapshot --clean --config $(GORELEASER_CONFIG)

docker:
	docker buildx build --platform $(DOCKER_PLATFORMS) --load --tag $(DOCKER_IMAGE) .

db-reset:
	rm -f $(DEV_DB)

db-shell:
	sqlite3 $(DEV_DB)
