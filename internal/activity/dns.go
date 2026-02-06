package activity

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"NYCU-SDC/deployment-service/internal/resolver"
	"context"

	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"
)

// DNSActivity handles DNS-related activities
type DNSActivity struct {
	dnsProvider domain.DNSProvider
	ipResolver  *resolver.IPResolver
	logger      *zap.Logger
}

// NewDNSActivity creates a new DNS activity
func NewDNSActivity(dnsProvider domain.DNSProvider, ipResolver *resolver.IPResolver, logger *zap.Logger) *DNSActivity {
	return &DNSActivity{
		dnsProvider: dnsProvider,
		ipResolver:  ipResolver,
		logger:      logger,
	}
}

// EnsureDNSRecord ensures a DNS A record exists
func (a *DNSActivity) EnsureDNSRecord(ctx context.Context, domain, ipPlaceholder string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Ensuring DNS record",
		zap.String("domain", domain),
		zap.String("ip_placeholder", ipPlaceholder),
	)

	// Resolve IP placeholder to actual IP address
	ip, err := a.ipResolver.Resolve(ipPlaceholder)
	if err != nil {
		logger.Error("Failed to resolve IP placeholder",
			zap.Error(err),
			zap.String("placeholder", ipPlaceholder),
		)
		return err
	}

	logger.Info("Resolved IP placeholder",
		zap.String("placeholder", ipPlaceholder),
		zap.String("ip", ip),
	)

	if err := a.dnsProvider.EnsureRecord(ctx, domain, ip); err != nil {
		logger.Error("Failed to ensure DNS record",
			zap.Error(err),
			zap.String("domain", domain),
			zap.String("ip", ip),
		)
		return err
	}

	logger.Info("DNS record ensured successfully",
		zap.String("domain", domain),
		zap.String("ip", ip),
	)

	return nil
}

// RemoveDNSRecord removes a DNS A record
func (a *DNSActivity) RemoveDNSRecord(ctx context.Context, domain string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Removing DNS record",
		zap.String("domain", domain),
	)

	if err := a.dnsProvider.RemoveRecord(ctx, domain); err != nil {
		logger.Error("Failed to remove DNS record",
			zap.Error(err),
			zap.String("domain", domain),
		)
		return err
	}

	logger.Info("DNS record removed successfully",
		zap.String("domain", domain),
	)

	return nil
}
