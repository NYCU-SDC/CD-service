package infisical

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Client implements domain.SecretManager interface
type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
	logger       *zap.Logger
	cache        *secretCache
}

type secretCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	secrets   map[string]string
	expiresAt time.Time
}

const cacheTTL = 5 * time.Minute

// NewClient creates a new Infisical client
func NewClient(baseURL, serviceToken string, logger *zap.Logger) *Client {
	return &Client{
		baseURL:      baseURL,
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
		cache: &secretCache{
			items: make(map[string]cacheItem),
		},
	}
}

// FetchSecrets fetches secrets from Infisical
func (c *Client) FetchSecrets(ctx context.Context, projectID, environment string, secretPaths []string) (map[string]string, error) {
	cacheKey := fmt.Sprintf("%s:%s:%v", projectID, environment, secretPaths)
	
	// Check cache
	c.cache.mu.RLock()
	if item, ok := c.cache.items[cacheKey]; ok {
		if time.Now().Before(item.expiresAt) {
			c.cache.mu.RUnlock()
			c.logger.Debug("Returning secrets from cache", zap.String("cache_key", cacheKey))
			return item.secrets, nil
		}
	}
	c.cache.mu.RUnlock()

	// Fetch from API
	secrets, err := c.fetchFromAPI(ctx, projectID, environment, secretPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secrets from Infisical: %w", err)
	}

	// Update cache
	c.cache.mu.Lock()
	c.cache.items[cacheKey] = cacheItem{
		secrets:   secrets,
		expiresAt: time.Now().Add(cacheTTL),
	}
	c.cache.mu.Unlock()

	return secrets, nil
}

func (c *Client) fetchFromAPI(ctx context.Context, projectID, environment string, secretPaths []string) (map[string]string, error) {
	// Infisical API endpoint for fetching secrets
	url := fmt.Sprintf("%s/api/v3/secrets", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.serviceToken))
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters
	q := req.URL.Query()
	q.Set("projectId", projectID)
	q.Set("environment", environment)
	if len(secretPaths) > 0 {
		// If secret paths are specified, fetch only those
		// This is a simplified implementation - adjust based on actual Infisical API
		for _, path := range secretPaths {
			q.Add("secretPath", path)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Infisical API returned status %d", resp.StatusCode)
	}

	var apiResponse struct {
		Secrets []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"secrets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	secrets := make(map[string]string)
	for _, secret := range apiResponse.Secrets {
		secrets[secret.Key] = secret.Value
	}

	return secrets, nil
}

// FetchSecretsByMapping fetches secrets from Infisical based on secret mappings
func (c *Client) FetchSecretsByMapping(ctx context.Context, project, environment string, mappings []domain.SecretMapping) (map[string]string, error) {
	result := make(map[string]string)
	
	for _, mapping := range mappings {
		// Fetch secrets for this specific path
		secrets, err := c.FetchSecrets(ctx, project, environment, []string{mapping.Path})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secrets for path %s: %w", mapping.Path, err)
		}
		
		// Find the secret by name and map it to the environment variable name
		if secretValue, ok := secrets[mapping.SecretName]; ok {
			result[mapping.EnvName] = secretValue
		} else {
			return nil, fmt.Errorf("secret %s not found in path %s", mapping.SecretName, mapping.Path)
		}
	}
	
	return result, nil
}

// Ensure Client implements domain.SecretManager
var _ domain.SecretManager = (*Client)(nil)
