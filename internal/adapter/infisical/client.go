package infisical

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
func (c *Client) FetchSecretsByMapping(ctx context.Context, workspaceSlug, environment string, mappings []domain.SecretMapping) (map[string]string, error) {
	result := make(map[string]string)

	for _, mapping := range mappings {
		// Fetch individual secret using the new API format
		secretValue, err := c.fetchSecretRaw(ctx, workspaceSlug, environment, mapping.SecretName, mapping.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secret %s from path %s: %w", mapping.SecretName, mapping.Path, err)
		}

		result[mapping.EnvName] = secretValue
	}

	return result, nil
}

// fetchSecretRaw fetches a single secret from Infisical using the raw API endpoint
func (c *Client) fetchSecretRaw(ctx context.Context, workspaceSlug, environment, secretName, secretPath string) (string, error) {
	// Build cache key
	cacheKey := fmt.Sprintf("%s:%s:%s:%s", workspaceSlug, environment, secretPath, secretName)

	// Check cache
	c.cache.mu.RLock()
	if item, ok := c.cache.items[cacheKey]; ok {
		if time.Now().Before(item.expiresAt) {
			c.cache.mu.RUnlock()
			c.logger.Debug("Returning secret from cache", zap.String("cache_key", cacheKey))
			// Extract the secret value from cache (cache stores map[string]string, but we only need one value)
			if secretValue, ok := item.secrets[secretName]; ok {
				return secretValue, nil
			}
		}
	}
	c.cache.mu.RUnlock()

	// Build API URL: /api/v3/secrets/raw/{secret_name}
	// Normalize base URL to remove trailing slash if present
	baseURL := c.baseURL
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	url := fmt.Sprintf("%s/api/v3/secrets/raw/%s", baseURL, secretName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.serviceToken))
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters: environment, workspaceSlug, secretPath, expandSecretReferences
	q := req.URL.Query()
	q.Set("environment", environment)
	q.Set("workspaceSlug", workspaceSlug)
	q.Set("secretPath", secretPath)
	q.Set("expandSecretReferences", "true")
	req.URL.RawQuery = q.Encode()

	c.logger.Debug("Fetching secret from Infisical",
		zap.String("url", req.URL.String()),
		zap.String("workspace_slug", workspaceSlug),
		zap.String("environment", environment),
		zap.String("secret_name", secretName),
		zap.String("secret_path", secretPath),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the full response body first to check for errors
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Log the actual response for debugging
		c.logger.Error("Infisical API error response",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", string(bodyBytes)),
			zap.String("url", req.URL.String()),
		)
		return "", fmt.Errorf("Infisical API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Try to parse as JSON first (expected format)
	var apiResponse struct {
		Secret struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"secret"`
	}

	var secretValue string
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		// If JSON parsing fails, check if it's a plain string response
		// Some raw API endpoints return just the value as a string
		if json.Valid(bodyBytes) {
			// Try alternative JSON format: direct value or different structure
			var altResponse struct {
				Value string `json:"value"`
			}
			if err2 := json.Unmarshal(bodyBytes, &altResponse); err2 == nil && altResponse.Value != "" {
				secretValue = altResponse.Value
			} else {
				// Log the actual response for debugging
				c.logger.Error("Failed to parse Infisical API response",
					zap.Error(err),
					zap.String("response_body", string(bodyBytes)),
					zap.String("url", req.URL.String()),
				)
				return "", fmt.Errorf("failed to decode response: %w (response: %s)", err, string(bodyBytes))
			}
		} else {
			// Response is not valid JSON - might be HTML error page or plain text
			previewLen := len(bodyBytes)
			if previewLen > 200 {
				previewLen = 200
			}
			c.logger.Error("Infisical API returned non-JSON response",
				zap.Error(err),
				zap.String("response_body", string(bodyBytes)),
				zap.String("url", req.URL.String()),
				zap.String("content_type", resp.Header.Get("Content-Type")),
			)
			return "", fmt.Errorf("failed to decode response: invalid JSON (got HTML/text?): %w (response preview: %s)", err, string(bodyBytes[:previewLen]))
		}
	} else {
		secretValue = apiResponse.Secret.Value
	}

	// Update cache
	c.cache.mu.Lock()
	c.cache.items[cacheKey] = cacheItem{
		secrets: map[string]string{
			secretName: secretValue,
		},
		expiresAt: time.Now().Add(cacheTTL),
	}
	c.cache.mu.Unlock()

	return secretValue, nil
}

// Ensure Client implements domain.SecretManager
var _ domain.SecretManager = (*Client)(nil)
