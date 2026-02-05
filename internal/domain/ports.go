package domain

import "context"

// SecretManager interface for managing secrets from Infisical
type SecretManager interface {
	// FetchSecrets fetches secrets from Infisical for the given project and environment
	// Deprecated: Use FetchSecretsByMapping instead
	FetchSecrets(ctx context.Context, projectID, environment string, secretPaths []string) (map[string]string, error)
	
	// FetchSecretsByMapping fetches secrets from Infisical based on secret mappings
	// Returns a map of environment variable names to secret values
	FetchSecretsByMapping(ctx context.Context, project, environment string, mappings []SecretMapping) (map[string]string, error)
}

// SSHExecutor interface for executing SSH operations
type SSHExecutor interface {
	// Execute executes a command on a remote host via SSH
	Execute(ctx context.Context, host string, user string, privateKey []byte, command string, envVars map[string]string) (string, error)
}

// DNSProvider interface for managing DNS records
type DNSProvider interface {
	// EnsureRecord ensures a DNS A record exists with the given domain and IP
	EnsureRecord(ctx context.Context, domain, ip string) error
	
	// RemoveRecord removes a DNS A record for the given domain
	RemoveRecord(ctx context.Context, domain string) error
}

// Notifier interface for sending notifications
type Notifier interface {
	// SendNotification sends a notification with the given message and status
	SendNotification(ctx context.Context, title, message string, success bool, metadata map[string]string) error
}
