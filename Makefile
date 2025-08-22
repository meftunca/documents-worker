# Documents Worker Makefile

# Variables
APP_NAME := documents-worker
VERSION := 1.0.0
DOCKER_IMAGE := $(APP_NAME):$(VERSION)
DOCKER_REGISTRY := your-registry.com  # Replace with your registry
K8S_NAMESPACE := default

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

.PHONY: help build clean test deps docker-build docker-push k8s-deploy k8s-delete dev

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(APP_NAME) .
	@echo "Build complete!"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(APP_NAME)
	@echo "Clean complete!"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run ./...

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) .
	docker tag $(DOCKER_IMAGE) $(APP_NAME):latest
	@echo "Docker image built successfully!"

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing Docker image to registry..."
	docker tag $(DOCKER_IMAGE) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	docker tag $(APP_NAME):latest $(DOCKER_REGISTRY)/$(APP_NAME):latest
	docker push $(DOCKER_REGISTRY)/$(APP_NAME):latest
	@echo "Docker image pushed successfully!"

docker-run: docker-build ## Run with Docker Compose
	@echo "Starting services with Docker Compose..."
	docker-compose up -d
	@echo "Services started! Check logs with: docker-compose logs -f"

docker-stop: ## Stop Docker Compose services
	@echo "Stopping services..."
	docker-compose down
	@echo "Services stopped!"

docker-logs: ## Show Docker Compose logs
	docker-compose logs -f

k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes namespace $(K8S_NAMESPACE)..."
	kubectl apply -f k8s/redis.yaml -n $(K8S_NAMESPACE)
	kubectl apply -f k8s/deployment.yaml -n $(K8S_NAMESPACE)
	@echo "Deployment complete!"
	@echo "Check status with: kubectl get pods -n $(K8S_NAMESPACE)"

k8s-delete: ## Delete from Kubernetes
	@echo "Deleting from Kubernetes namespace $(K8S_NAMESPACE)..."
	kubectl delete -f k8s/deployment.yaml -n $(K8S_NAMESPACE) || true
	kubectl delete -f k8s/redis.yaml -n $(K8S_NAMESPACE) || true
	@echo "Resources deleted!"

k8s-status: ## Check Kubernetes deployment status
	@echo "Checking deployment status..."
	kubectl get pods,svc,configmap,secret -l app=documents-worker -n $(K8S_NAMESPACE)
	kubectl get pods,svc -l app=redis -n $(K8S_NAMESPACE)

k8s-logs: ## Show Kubernetes logs
	kubectl logs -l app=documents-worker -n $(K8S_NAMESPACE) --tail=100 -f

k8s-port-forward: ## Port forward for local access
	@echo "Port forwarding documents-worker service to localhost:3001..."
	kubectl port-forward svc/documents-worker-service 3001:80 -n $(K8S_NAMESPACE)

dev: ## Run in development mode
	@echo "Starting development environment..."
	docker-compose up -d redis
	@echo "Waiting for Redis to be ready..."
	sleep 5
	@echo "Starting application..."
	ENVIRONMENT=development \
	REDIS_HOST=localhost \
	REDIS_PORT=6379 \
	WORKER_MAX_CONCURRENCY=3 \
	./$(APP_NAME)

run: build ## Build and run locally
	@echo "Running $(APP_NAME)..."
	./$(APP_NAME)

install-tools: ## Install development tools
	@echo "Installing development tools..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/air-verse/air@latest

air: ## Run with hot reload
	@echo "Starting with hot reload..."
	air

generate-docs: ## Generate API documentation
	@echo "Generating API documentation..."
	# Add your API doc generation command here
	@echo "Documentation generated!"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

profile: ## Run with profiling
	@echo "Running with profiling enabled..."
	$(GOBUILD) -tags profile -o $(APP_NAME)-profile .
	./$(APP_NAME)-profile

security-scan: ## Run security scan
	@echo "Running security scan..."
	govulncheck ./...

all: clean deps test build ## Clean, get dependencies, test, and build

# Development shortcuts
redis: ## Start only Redis for development
	docker-compose up -d redis

stop-redis: ## Stop Redis
	docker-compose stop redis

reset-redis: ## Reset Redis data
	docker-compose down -v
	docker-compose up -d redis
