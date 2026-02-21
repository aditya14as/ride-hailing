.PHONY: setup run test docker-up docker-down migrate-up migrate-down seed build clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
BINARY_NAME=gocomet

# Setup dependencies
setup:
	$(GOCMD) mod tidy

# Build the binary
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) cmd/server/main.go

# Run the server
run:
	$(GOCMD) run cmd/server/main.go

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Docker commands
docker-up:
	docker-compose -f docker/docker-compose.yml up -d

docker-down:
	docker-compose -f docker/docker-compose.yml down

docker-logs:
	docker-compose -f docker/docker-compose.yml logs -f

docker-clean:
	docker-compose -f docker/docker-compose.yml down -v

# Database migrations
migrate-up:
	@echo "Running migrations..."
	@docker exec -i gocomet-postgres psql -U gocomet -d gocomet < migrations/001_initial_schema.up.sql

migrate-down:
	@echo "Rolling back migrations..."
	@docker exec -i gocomet-postgres psql -U gocomet -d gocomet < migrations/001_initial_schema.down.sql

# Seed test data
seed:
	$(GOCMD) run scripts/seed_data.go

# Load testing
load-test:
	$(GOCMD) run scripts/load_test.go

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint code
lint:
	golangci-lint run

# All in one: start fresh
fresh: docker-down docker-clean docker-up
	@echo "Waiting for services to start..."
	@sleep 5
	@make migrate-up
	@echo "Ready! Run 'make run' to start the server"
