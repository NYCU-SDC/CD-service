package cloudflare

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Client implements domain.DNSProvider interface
type Client struct {
	apiToken   string
	zoneID     string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new Cloudflare client
func NewClient(apiToken, zoneID string, logger *zap.Logger) *Client {
	return &Client{
		apiToken:   apiToken,
		zoneID:     zoneID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// DNSRecord represents a Cloudflare DNS record
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type listDNSRecordsResponse struct {
	Result  []DNSRecord `json:"result"`
	Success bool        `json:"success"`
}

type createDNSRecordResponse struct {
	Result  DNSRecord `json:"result"`
	Success bool      `json:"success"`
}

// EnsureRecord ensures a DNS A record exists with the given domain and IP
func (c *Client) EnsureRecord(ctx context.Context, domain, ip string) error {
	// Check if record already exists
	existingRecord, err := c.findRecord(ctx, domain)
	if err != nil {
		return fmt.Errorf("failed to find existing record: %w", err)
	}

	if existingRecord != nil {
		// Record exists, check if IP matches
		if existingRecord.Content == ip {
			c.logger.Info("DNS record already exists with correct IP",
				zap.String("domain", domain),
				zap.String("ip", ip),
			)
			return nil
		}
		// IP doesn't match, update the record
		return c.updateRecord(ctx, existingRecord.ID, domain, ip)
	}

	// Record doesn't exist, create it
	return c.createRecord(ctx, domain, ip)
}

// RemoveRecord removes a DNS A record for the given domain
func (c *Client) RemoveRecord(ctx context.Context, domain string) error {
	record, err := c.findRecord(ctx, domain)
	if err != nil {
		return fmt.Errorf("failed to find record: %w", err)
	}

	if record == nil {
		c.logger.Info("DNS record not found, nothing to remove",
			zap.String("domain", domain),
		)
		return nil
	}

	return c.deleteRecord(ctx, record.ID)
}

func (c *Client) findRecord(ctx context.Context, domain string) (*DNSRecord, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", c.zoneID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters
	q := req.URL.Query()
	q.Set("type", "A")
	q.Set("name", domain)
	req.URL.RawQuery = q.Encode()

	// Log request details
	c.logger.Debug("Sending Cloudflare API request",
		zap.String("method", "GET"),
		zap.String("url", req.URL.String()),
		zap.String("zone_id", c.zoneID),
		zap.String("domain", domain),
		zap.String("token_prefix", maskToken(c.apiToken)),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to send Cloudflare API request",
			zap.Error(err),
			zap.String("url", req.URL.String()),
		)
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Cloudflare API returned error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("url", req.URL.String()),
			zap.String("response_body", string(bodyBytes)),
			zap.String("token_prefix", maskToken(c.apiToken)),
		)
		return nil, fmt.Errorf("Cloudflare API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResponse listDNSRecordsResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		return nil, fmt.Errorf("Cloudflare API returned success=false")
	}

	if len(apiResponse.Result) == 0 {
		c.logger.Debug("No DNS record found",
			zap.String("domain", domain),
		)
		return nil, nil
	}

	c.logger.Debug("DNS record found",
		zap.String("domain", domain),
		zap.String("record_id", apiResponse.Result[0].ID),
		zap.String("content", apiResponse.Result[0].Content),
	)

	return &apiResponse.Result[0], nil
}

func (c *Client) createRecord(ctx context.Context, domain, ip string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", c.zoneID)

	payload := map[string]interface{}{
		"type":    "A",
		"name":    domain,
		"content": ip,
		"ttl":     1, // Auto TTL
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Creating DNS record",
		zap.String("url", url),
		zap.String("domain", domain),
		zap.String("ip", ip),
		zap.String("token_prefix", maskToken(c.apiToken)),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to create DNS record",
			zap.Error(err),
			zap.String("domain", domain),
		)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Error("Cloudflare API returned error when creating DNS record",
			zap.Int("status_code", resp.StatusCode),
			zap.String("domain", domain),
			zap.String("ip", ip),
			zap.String("response_body", string(bodyBytes)),
			zap.String("token_prefix", maskToken(c.apiToken)),
		)
		return fmt.Errorf("Cloudflare API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResponse createDNSRecordResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		return fmt.Errorf("Cloudflare API returned success=false")
	}

	c.logger.Info("DNS record created",
		zap.String("domain", domain),
		zap.String("ip", ip),
	)

	return nil
}

func (c *Client) updateRecord(ctx context.Context, recordID, domain, ip string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", c.zoneID, recordID)

	payload := map[string]interface{}{
		"type":    "A",
		"name":    domain,
		"content": ip,
		"ttl":     1, // Auto TTL
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Updating DNS record",
		zap.String("url", url),
		zap.String("record_id", recordID),
		zap.String("domain", domain),
		zap.String("ip", ip),
		zap.String("token_prefix", maskToken(c.apiToken)),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to update DNS record",
			zap.Error(err),
			zap.String("record_id", recordID),
		)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Cloudflare API returned error when updating DNS record",
			zap.Int("status_code", resp.StatusCode),
			zap.String("record_id", recordID),
			zap.String("domain", domain),
			zap.String("response_body", string(bodyBytes)),
			zap.String("token_prefix", maskToken(c.apiToken)),
		)
		return fmt.Errorf("Cloudflare API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResponse createDNSRecordResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		return fmt.Errorf("Cloudflare API returned success=false")
	}

	c.logger.Info("DNS record updated",
		zap.String("domain", domain),
		zap.String("ip", ip),
	)

	return nil
}

func (c *Client) deleteRecord(ctx context.Context, recordID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", c.zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Deleting DNS record",
		zap.String("url", url),
		zap.String("record_id", recordID),
		zap.String("token_prefix", maskToken(c.apiToken)),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to delete DNS record",
			zap.Error(err),
			zap.String("record_id", recordID),
		)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Cloudflare API returned error when deleting DNS record",
			zap.Int("status_code", resp.StatusCode),
			zap.String("record_id", recordID),
			zap.String("response_body", string(bodyBytes)),
			zap.String("token_prefix", maskToken(c.apiToken)),
		)
		return fmt.Errorf("Cloudflare API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResponse struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		return fmt.Errorf("Cloudflare API returned success=false")
	}

	c.logger.Info("DNS record deleted",
		zap.String("record_id", recordID),
	)

	return nil
}

// maskToken masks the token for logging (shows first 8 and last 4 characters)
func maskToken(token string) string {
	if len(token) <= 12 {
		return "***"
	}
	return token[:8] + "..." + token[len(token)-4:]
}

// Ensure Client implements domain.DNSProvider
var _ domain.DNSProvider = (*Client)(nil)
