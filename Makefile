.PHONY: run build clean test deps help stop logs ps fmt lint install

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=bot
MAIN_PATH=cmd/bot/main.go
COMPOSE_SCRIPT=sh ./scripts/compose.sh
RUN_SCRIPT=sh ./scripts/run.sh

## help: Ko'rsatish barcha mavjud komandalar
help:
	@echo "Mavjud komandalar:"
	@echo "  make run     - Bot + database ni ishga tushirish (chiroyli output)"
	@echo "  make ps      - Docker konteynerlar holati"
	@echo "  make logs    - Docker loglarini ko'rish"
	@echo "  make stop    - Docker konteynerlarni to'xtatish"
	@echo "  make build   - Botni build qilish"
	@echo "  make clean   - Build fayllarni o'chirish"
	@echo "  make deps    - Dependencies ni o'rnatish"
	@echo "  make test    - Testlarni ishga tushirish"
	@echo "  make fmt     - Kodni formatlash"
	@echo "  make lint    - Kodni tekshirish"

## run: Botni ishga tushirish
run:
	@$(RUN_SCRIPT)

## ps: Docker konteynerlar holati
ps:
	@$(COMPOSE_SCRIPT) ps

## logs: Docker loglarini ko'rish
logs:
	@$(COMPOSE_SCRIPT) logs -f --tail=200

## build: Botni build qilish
build:
	@echo "Build qilinyapti..."
	@go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build tayyor: ./$(BINARY_NAME)"

## clean: Build fayllarni o'chirish
clean:
	@echo "Tozalanyapti..."
	@rm -f $(BINARY_NAME)
	@go clean
	@echo "Tozalandi!"

## deps: Dependencies ni o'rnatish
deps:
	@echo "Dependencies o'rnatilmoqda..."
	@go mod download
	@go mod tidy
	@echo "Dependencies tayyor!"

## test: Testlarni ishga tushirish
test:
	@echo "Testlar ishga tushmoqda..."
	@go test -v ./...

## fmt: Kodni formatlash
fmt:
	@echo "Kod formatlanmoqda..."
	@go fmt ./...
	@echo "Format tayyor!"

## stop: Docker konteynerlarni to'xtatish
stop:
	@echo "Docker konteynerlar to'xtatilmoqda..."
	@$(COMPOSE_SCRIPT) down

## lint: Kodni tekshirish (golangci-lint kerak)
lint:
	@echo "Kod tekshirilmoqda..."
	@golangci-lint run ./...

## install: Binary ni install qilish
install: build
	@echo "Installing..."
	@go install $(MAIN_PATH)
