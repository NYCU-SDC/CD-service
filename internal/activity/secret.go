package activity

import (
	"NYCU-SDC/deployment-service/internal/domain"
	"context"

	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"
)

// SecretActivity handles secret-related activities
type SecretActivity struct {
	secretManager domain.SecretManager
	logger        *zap.Logger
}

// NewSecretActivity creates a new secret activity
func NewSecretActivity(secretManager domain.SecretManager, logger *zap.Logger) *SecretActivity {
	return &SecretActivity{
		secretManager: secretManager,
		logger:        logger,
	}
}

// FetchInfisicalSecrets fetches secrets from Infisical using secret mappings
func (a *SecretActivity) FetchInfisicalSecrets(ctx context.Context, project, environment string, mappings []domain.SecretMapping) (map[string]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching secrets from Infisical",
		zap.String("project", project),
		zap.String("environment", environment),
		zap.Int("mapping_count", len(mappings)),
	)

	secrets, err := a.secretManager.FetchSecretsByMapping(ctx, project, environment, mappings)
	if err != nil {
		logger.Error("Failed to fetch secrets",
			zap.Error(err),
			zap.String("project", project),
			zap.String("environment", environment),
		)
		return nil, err
	}

	logger.Info("Successfully fetched secrets",
		zap.Int("count", len(secrets)),
		zap.String("project", project),
		zap.String("environment", environment),
	)

	return secrets, nil
}
