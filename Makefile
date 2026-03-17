.PHONY: dev build test lint clean

# Local development
dev:
	docker compose up --build

dev-api:
	cd services/api && go run ./cmd/main.go

dev-portal:
	cd packages/portal && npm run dev

dev-link:
	cd packages/link && npm run dev

# Build
build-api:
	cd services/api && go build -o bin/api ./cmd/main.go

build-link:
	cd packages/link && npm run build

build-portal:
	cd packages/portal && npm run build

build-enrichment:
	cd services/enrichment && docker build -t hound-enrichment .

build: build-api build-link build-portal

# Test
test-api:
	cd services/api && go test ./... -v -race

test-link:
	cd packages/link && npm test

test: test-api test-link

# Lint
lint-api:
	cd services/api && golangci-lint run ./...

lint-link:
	cd packages/link && npm run lint

lint: lint-api lint-link

# Database
db-migrate:
	cd services/api && go run ./cmd/migrate/main.go up

db-rollback:
	cd services/api && go run ./cmd/migrate/main.go down

# Terraform
tf-init:
	cd terraform/environments/staging && terraform init

tf-plan:
	cd terraform/environments/staging && terraform plan

tf-apply:
	cd terraform/environments/staging && terraform apply

# Clean
clean:
	rm -f services/api/bin/*
	rm -rf packages/link/dist
	rm -rf packages/portal/.next
