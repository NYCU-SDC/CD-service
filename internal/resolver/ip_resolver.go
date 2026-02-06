package resolver

import (
	"fmt"

	"go.uber.org/zap"
)

// IPResolver resolves IP address placeholders to actual IP addresses
type IPResolver struct {
	mappings map[string]string
	logger   *zap.Logger
}

// NewIPResolver creates a new IP resolver with the given mappings
func NewIPResolver(mappings map[string]string, logger *zap.Logger) *IPResolver {
	return &IPResolver{
		mappings: mappings,
		logger:   logger,
	}
}

// Resolve resolves a placeholder to an IP address
// Returns the IP address if found in mappings, otherwise returns an error
func (r *IPResolver) Resolve(placeholder string) (string, error) {
	ip, found := r.mappings[placeholder]
	if !found {
		r.logger.Error("IP placeholder not found in mappings",
			zap.String("placeholder", placeholder),
			zap.Int("available_mappings", len(r.mappings)),
		)
		return "", fmt.Errorf("IP placeholder '%s' not found in mappings", placeholder)
	}

	r.logger.Debug("Resolved IP placeholder",
		zap.String("placeholder", placeholder),
		zap.String("ip", ip),
	)

	return ip, nil
}
