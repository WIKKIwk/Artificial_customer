.PHONY: up up-detached down logs run run-new test test-verbose help migrate clean

DOCKER_COMPOSE := $(shell if command -v docker-compose >/dev/null 2>&1; then echo docker-compose; else echo "docker compose"; fi)

up:
	$(DOCKER_COMPOSE) up --build

up-detached:
	$(DOCKER_COMPOSE) up -d --build
	@echo "Stack started!"
	@echo "Frontend: http://localhost:8001"
	@echo "Go API: http://localhost:8080"
	@echo "Elixir realtime: ws://localhost:4000/socket"
	@echo "Health check: http://localhost:8080/health"

down:
	$(DOCKER_COMPOSE) down
	@echo "Application stopped."

logs:
	$(DOCKER_COMPOSE) logs -f

run:
	@echo "Running Go API with v2.3.0 features..."
	cd backend-go && go run main.go

run-legacy:
	@echo "Running legacy main.go (backup)..."
	cd backend-go && go run main_legacy.go.bak

test:
	@echo "Running tests..."
	cd backend-go && go test ./... -cover

test-verbose:
	@echo "Running tests with verbose output..."
	cd backend-go && go test ./... -v -cover

migrate:
	@echo "Running database migrations..."
	@echo "Migrations are auto-run on startup via gorm.AutoMigrate"

clean:
	@echo "Cleaning up..."
	cd backend-go && go clean
	$(DOCKER_COMPOSE) down -v
	@echo "Cleanup complete!"

help:
	@echo "Available commands:"
	@echo "  make up              - Start all services with Docker Compose"
	@echo "  make up-detached     - Start services in background"
	@echo "  make down            - Stop all services"
	@echo "  make logs            - View logs"
	@echo "  make run             - Run Go API with v2.3.0 features"
	@echo "  make run-legacy      - Run legacy version (backup)"
	@echo "  make test            - Run unit tests"
	@echo "  make test-verbose    - Run tests with verbose output"
	@echo "  make clean           - Clean up containers and volumes"
	@echo "  make help            - Show this help message"
