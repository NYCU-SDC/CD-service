package workflow

import (
	"NYCU-SDC/deployment-service/internal/activity"
	"NYCU-SDC/deployment-service/internal/domain"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CDWorkflow orchestrates the CD deployment process
func CDWorkflow(ctx workflow.Context, req domain.DeployRequest) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("CD Workflow started",
		"project", req.Metadata.ProjectName,
		"environment", req.Metadata.Environment,
		"method", string(req.Method),
		"trace_id", req.TraceID,
	)

	// Configure Activity Options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Fetch Secrets (if enabled)
	var secrets map[string]string
	if req.Setup.InjectSecret.Enable {
		logger.Info("Fetching secrets from Infisical")
		err := workflow.ExecuteActivity(ctx, activity.ActivityFetchInfisicalSecrets,
			req.Setup.InjectSecret.Project,
			req.Setup.InjectSecret.Environment,
			req.Setup.InjectSecret.Secrets,
		).Get(ctx, &secrets)
		if err != nil {
			logger.Error("Failed to fetch secrets", "error", err)
			// Send failure notification
			if notifyErr := workflow.ExecuteActivity(ctx, activity.ActivitySendDiscordNotification, req, "Failed to fetch secrets", err).Get(ctx, nil); notifyErr != nil {
				logger.Error("Failed to send failure notification", "error", notifyErr)
			}
			return err
		}
		logger.Info("Secrets fetched successfully", "count", len(secrets))
	}

	// Step 2: Execute SSH Deployment/Cleanup
	var deployOutput string
	err := workflow.ExecuteActivity(ctx, activity.ActivityRunSSHDeploy, req, secrets).Get(ctx, &deployOutput)
	if err != nil {
		logger.Error("SSH deployment failed", "error", err)
		// Send failure notification
		if notifyErr := workflow.ExecuteActivity(ctx, activity.ActivitySendDiscordNotification, req, "Deployment Failed", err).Get(ctx, nil); notifyErr != nil {
			logger.Error("Failed to send failure notification", "error", notifyErr)
		}
		return err
	}
	logger.Info("SSH deployment completed successfully")

	// Step 3: Handle DNS (if enabled)
	if req.Method == domain.MethodDeploy && req.Post.SetupDomain.Enable {
		if req.Post.SetupDomain.Name != "" && req.Post.SetupDomain.Value != "" {
			logger.Info("Setting up DNS record",
				"name", req.Post.SetupDomain.Name,
				"value", req.Post.SetupDomain.Value,
			)
			// Extract IP from value (if it's a service:port format, we'll need to resolve it)
			// For now, assume value is an IP address
			ip := req.Post.SetupDomain.Value
			err := workflow.ExecuteActivity(ctx, activity.ActivityEnsureDNSRecord,
				req.Post.SetupDomain.Name,
				ip,
			).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to setup DNS record", "error", err)
				// Don't fail the workflow if DNS setup fails, but log it
			}
		}
	} else if req.Method == domain.MethodCleanup && req.Post.CleanupDomain.Enable {
		if req.Post.CleanupDomain.Name != "" {
			logger.Info("Cleaning up DNS record", "name", req.Post.CleanupDomain.Name)
			err := workflow.ExecuteActivity(ctx, activity.ActivityRemoveDNSRecord, req.Post.CleanupDomain.Name).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to cleanup DNS record", "error", err)
				// Don't fail the workflow if DNS cleanup fails, but log it
			}
		}
	}

	// Step 4: Send success notification
	if req.Post.NotifyDiscord.Enable {
		logger.Info("Sending success notification")
		if err := workflow.ExecuteActivity(ctx, activity.ActivitySendDiscordNotification, req, "Deployment Successful", nil).Get(ctx, nil); err != nil {
			logger.Error("Failed to send success notification", "error", err)
			// Don't fail the workflow if notification fails, but log it
		}
	}

	logger.Info("CD Workflow completed successfully")
	return nil
}
