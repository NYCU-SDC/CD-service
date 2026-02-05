# Deployment Service

A CD (Continuous Deployment) service built with Go and Temporal SDK.

## Features

- Webhook API for receiving deployment requests from GitHub Actions
- Temporal workflow orchestration for reliable CD processes
- Integration with Infisical for secret management
- SSH-based deployment execution
- Cloudflare DNS management
- Discord notifications
- Full observability with OpenTelemetry and structured logging

## Architecture

The service follows a hexagonal architecture pattern:

- **Domain Layer**: Core business logic and interfaces
- **Workflow Layer**: Temporal workflow orchestration
- **Activity Layer**: Individual deployment steps
- **Adapter Layer**: External service integrations
- **API Layer**: HTTP handlers and middleware

## Project Structure

```
deployment-service/
├── cmd/
│   ├── api/          # API server entry point
│   └── worker/       # Temporal worker entry point
├── internal/
│   ├── domain/       # Domain models and interfaces
│   ├── workflow/     # Temporal workflows
│   ├── activity/     # Temporal activities
│   ├── adapter/      # External service adapters
│   ├── config/       # Configuration management
│   ├── handler/      # HTTP handlers
│   ├── middleware/   # HTTP middleware
│   └── logger/       # Logger utilities
├── config.example.yaml
├── docker-compose.yaml          # API and Worker services
├── docker-compose.temporal.yaml # Temporal infrastructure
└── Dockerfile
```

## Configuration

Copy `config.example.yaml` to `config.yaml` and configure:

- Temporal server address and namespace
- Deploy token for webhook authentication
- Infisical credentials
- Cloudflare API token and zone ID
- Discord webhook URL
- OpenTelemetry collector URL
- SSH configuration (host, user, port, private_key)

### SSH Private Key Configuration

SSH private key must be configured via `private_key` field in `config.yaml` or `SSH_PRIVATE_KEY` environment variable. Multi-line private keys are supported using YAML literal block scalar (`|`):

```yaml
ssh:
  private_key: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

Or via environment variable:
```bash
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_ed25519)"
```

## Running Locally

### Step 1: Start Temporal Infrastructure

Start Temporal Server, PostgreSQL, and UI:

```bash
docker-compose -f docker-compose.temporal.yaml up -d
```

This will start:
- Temporal Server on port `7233`
- Temporal UI on port `8080`
- PostgreSQL for Temporal on port `5432`

### Step 2: Start API and Worker Services

Option A: Using Docker Compose

```bash
docker-compose up -d
```

Option B: Running Locally

```bash
# Start API
go run cmd/api/main.go

# Start Worker (in another terminal)
go run cmd/worker/main.go
```

## Service Ports

- **Temporal UI**: `http://localhost:8080`
- **Temporal Server**: `localhost:7233`
- **API Service**: `http://localhost:8082`

## API Endpoints

### POST /api/webhook/deploy

Deploy or cleanup a service.

**Headers:**
- `x-deploy-token`: Authentication token

**Request Body:**
```json
{
  "source": {
    "title": "Core System",
    "repo": "NYCU-SDC/core-system-backend",
    "branch": "main",
    "commit": "a58327e5a861d8e4bb7ccc75a324ae97caf8c089",
    "pr_number": "123",
    "pr_title": "CD test",
    "pr_type": "Test",
    "pr_purpose": "Test CD workflow"
  },
  "method": "deploy",
  "metadata": {
    "project_name": "core-system",
    "component": "backend",
    "environment": "stage"
  },
  "setup": {
    "inject_secret": {
      "enable": false,
      "project": "core-system",
      "environment": "snapshot",
      "secrets": [
        {
          "path": "/",
          "secret_name": "OAUTH_PROXY_TOKEN",
          "env_name": "OAUTH_PROXY_TOKEN"
        }
      ]
    }
  },
  "post": {
    "setup_domain": {
      "enable": true,
      "title": "Endpoint",
      "name": "stage.core-system.sdc.nycu.club",
      "value": "default-eng-deploy:internal"
    },
    "cleanup_domain": {
      "enable": false,
      "title": "",
      "name": "",
      "value": ""
    },
    "notify_discord": {
      "enable": true,
      "channel": "core-system-activity"
    }
  }
}
```

**Response:**
```json
{
  "workflow_id": "deploy-...",
  "run_id": "...",
  "trace_id": "...",
  "status": "started"
}
```

See `webhook-payload.deploy.json` and `webhook-payload.cleanup.json` for complete examples.

### GET /api/healthz

Health check endpoint.

## Observability

The service integrates with:

- **OpenTelemetry**: Traces sent to Tempo (if configured)
- **Structured Logging**: JSON logs compatible with Loki
- **Temporal UI**: Available at http://localhost:8080

## Testing Webhooks

Use the provided Makefile targets to test deployment workflows:

```bash
# Send deploy webhook
make deploy

# Send cleanup webhook
make cleanup

# Custom API URL and token
make deploy API_URL=http://your-api:8082 DEPLOY_TOKEN=your-token
make cleanup API_URL=http://your-api:8082 DEPLOY_TOKEN=your-token
```

Webhook payload examples:
- `webhook-payload.deploy.json` - Deploy workflow example
- `webhook-payload.cleanup.json` - Cleanup workflow example

## Development

### Dependencies

- Go 1.24+
- Temporal Server
- Docker and Docker Compose

### Building

```bash
# Build both API and Worker
make build

# Build individually
make build-api
make build-worker

# Run locally
make run-api
make run-worker
```

### Makefile Targets

- `make build` - Build API and Worker binaries
- `make build-api` - Build API binary only
- `make build-worker` - Build Worker binary only
- `make run-api` - Run API server locally
- `make run-worker` - Run Worker locally
- `make deploy` - Send deploy webhook request
- `make cleanup` - Send cleanup webhook request
- `make clean` - Remove built binaries

## License

MIT
