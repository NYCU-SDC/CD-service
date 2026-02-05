GREEN = \033[0;32m
BLUE = \033[0;34m
RED = \033[0;31m
NC = \033[0m

all: build

prepare:
	@echo -e ":: $(GREEN)Preparing environment...$(NC)"
	@echo -e ":: $(GREEN)Downloading go dependencies...$(NC)"
	@go mod download \
		&& echo -e "==> $(BLUE)Successfully downloaded go dependencies$(NC)" \
		|| (echo -e "==> $(RED)Failed to download go dependencies$(NC)" && exit 1)

build: build-api build-worker

build-api:
	@echo -e ":: $(GREEN)Building API...$(NC)"
	@echo -e "  -> Building API binary..."
	@go build -o bin/api cmd/api/main.go && echo -e "==> $(BLUE)API build completed successfully$(NC)" || (echo -e "==> $(RED)API build failed$(NC)" && exit 1)

build-worker:
	@echo -e ":: $(GREEN)Building Worker...$(NC)"
	@echo -e "  -> Building Worker binary..."
	@go build -o bin/worker cmd/worker/main.go && echo -e "==> $(BLUE)Worker build completed successfully$(NC)" || (echo -e "==> $(RED)Worker build failed$(NC)" && exit 1)

run-api:
	@echo -e ":: $(GREEN)Starting API...$(NC)"
	@go build -o bin/api cmd/api/main.go && \
		./bin/api \
		&& echo -e "==> $(BLUE)Successfully shut down API$(NC)" \
		|| (echo -e "==> $(RED)API failed to start$(NC)" && exit 1)

run-worker:
	@echo -e ":: $(GREEN)Starting Worker...$(NC)"
	@go build -o bin/worker cmd/worker/main.go && \
		./bin/worker \
		&& echo -e "==> $(BLUE)Successfully shut down Worker$(NC)" \
		|| (echo -e "==> $(RED)Worker failed to start$(NC)" && exit 1)

clean:
	@echo -e ":: $(GREEN)Cleaning binaries...$(NC)"
	@rm -f bin/api bin/worker && echo -e "==> $(BLUE)Clean completed$(NC)" || (echo -e "==> $(RED)Clean failed$(NC)" && exit 1)
	@rmdir bin 2>/dev/null || true

deploy:
	@echo -e ":: $(GREEN)Sending deploy webhook request...$(NC)"
	@PAYLOAD_FILE=webhook-payload.deploy.json; \
	API_URL_VAL=$${API_URL:-http://localhost:8082}; \
	./scripts/send-webhook.sh $$PAYLOAD_FILE $$API_URL_VAL $(DEPLOY_TOKEN)

cleanup:
	@echo -e ":: $(GREEN)Sending cleanup webhook request...$(NC)"
	@PAYLOAD_FILE=webhook-payload.cleanup.json; \
	API_URL_VAL=$${API_URL:-http://localhost:8082}; \
	./scripts/send-webhook.sh $$PAYLOAD_FILE $$API_URL_VAL $(DEPLOY_TOKEN)

.PHONY: all prepare build build-api build-worker run-api run-worker test deploy cleanup
