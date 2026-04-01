package ses

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"golang.org/x/sync/errgroup"
)

// ValidationCheck represents a single permission validation result.
type ValidationCheck struct {
	Name        string `json:"name"`
	Permission  string `json:"permission"`
	Status      string `json:"status"` // "ok", "denied", "error"
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ValidationResult holds all validation check results.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Checks []ValidationCheck `json:"checks"`
}

// ValidateCredentials tests SES/SNS permissions without creating any resources.
// All checks run concurrently for lower latency.
func ValidateCredentials(ctx context.Context, cfg Config) (*ValidationResult, error) {
	awsCfg, err := LoadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid AWS credentials: %w", err)
	}

	sesClient := sesv2.NewFromConfig(awsCfg, func(o *sesv2.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})
	snsClient := sns.NewFromConfig(awsCfg, func(o *sns.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})

	var (
		mu     sync.Mutex
		checks = make([]ValidationCheck, 8)
		valid  = true
	)

	setCheck := func(idx int, c ValidationCheck) {
		mu.Lock()
		checks[idx] = c
		if c.Required && c.Status != "ok" {
			valid = false
		}
		mu.Unlock()
	}

	g, gCtx := errgroup.WithContext(ctx)

	// 1. SES: ListEmailIdentities
	g.Go(func() error {
		c := ValidationCheck{
			Name: "ses_list_identities", Permission: "ses:ListEmailIdentities",
			Description: "List verified sender identities", Required: true, Status: "ok",
		}
		if _, err := sesClient.ListEmailIdentities(gCtx, &sesv2.ListEmailIdentitiesInput{PageSize: aws.Int32(1)}); err != nil {
			c.Status = "denied"
		}
		setCheck(0, c)
		return nil
	})

	// 2. SES: GetAccount
	g.Go(func() error {
		c := ValidationCheck{
			Name: "ses_account", Permission: "ses:GetAccount",
			Description: "Read SES account status (sandbox/production)", Required: true, Status: "ok",
		}
		if _, err := sesClient.GetAccount(gCtx, &sesv2.GetAccountInput{}); err != nil {
			c.Status = "denied"
		}
		setCheck(1, c)
		return nil
	})

	// 3. SES: ListConfigurationSets
	g.Go(func() error {
		c := ValidationCheck{
			Name: "ses_configuration_set", Permission: "ses:ListConfigurationSets",
			Description: "List configuration sets (validates tracking access)", Required: false, Status: "ok",
		}
		if _, err := sesClient.ListConfigurationSets(gCtx, &sesv2.ListConfigurationSetsInput{}); err != nil {
			c.Status = "denied"
		}
		setCheck(2, c)
		return nil
	})

	// 4. SNS: ListTopics
	g.Go(func() error {
		c := ValidationCheck{
			Name: "sns_topic", Permission: "sns:ListTopics",
			Description: "List SNS topics (validates tracking access)", Required: false, Status: "ok",
		}
		if _, err := snsClient.ListTopics(gCtx, &sns.ListTopicsInput{}); err != nil {
			c.Status = "denied"
		}
		setCheck(3, c)
		return nil
	})

	// 5. SES: DeleteConfigurationSet (non-existent resource triggers NotFoundException, not actual deletion)
	g.Go(func() error {
		c := ValidationCheck{
			Name: "ses_delete_configuration_set", Permission: "ses:DeleteConfigurationSet",
			Description: "Delete configuration sets (cleanup)", Required: false, Status: "ok",
		}
		_, err := sesClient.DeleteConfigurationSet(gCtx, &sesv2.DeleteConfigurationSetInput{
			ConfigurationSetName: aws.String("senda-validate-perm-check"),
		})
		if err != nil && IsAccessDenied(err) {
			c.Status = "denied"
		}
		setCheck(4, c)
		return nil
	})

	// 6. SES: DeleteConfigurationSetEventDestination (non-existent resource triggers NotFoundException)
	g.Go(func() error {
		c := ValidationCheck{
			Name: "ses_delete_event_destination", Permission: "ses:DeleteConfigurationSetEventDestination",
			Description: "Delete event destinations from configuration sets (cleanup)", Required: false, Status: "ok",
		}
		_, err := sesClient.DeleteConfigurationSetEventDestination(gCtx, &sesv2.DeleteConfigurationSetEventDestinationInput{
			ConfigurationSetName: aws.String("senda-validate-perm-check"),
			EventDestinationName: aws.String("senda-validate-perm-check"),
		})
		if err != nil && IsAccessDenied(err) {
			c.Status = "denied"
		}
		setCheck(5, c)
		return nil
	})

	// 7. SNS: Unsubscribe (non-existent ARN triggers InvalidParameter/NotFoundException)
	g.Go(func() error {
		c := ValidationCheck{
			Name: "sns_unsubscribe", Permission: "sns:Unsubscribe",
			Description: "Unsubscribe from SNS topics (cleanup)", Required: false, Status: "ok",
		}
		fakeARN := fmt.Sprintf("arn:aws:sns:%s:000000000000:senda-validate-perm-check", awsCfg.Region)
		_, err := snsClient.Unsubscribe(gCtx, &sns.UnsubscribeInput{
			SubscriptionArn: aws.String(fakeARN),
		})
		if err != nil && IsAccessDenied(err) {
			c.Status = "denied"
		}
		setCheck(6, c)
		return nil
	})

	// 8. SNS: DeleteTopic (non-existent ARN triggers NotFoundException)
	g.Go(func() error {
		c := ValidationCheck{
			Name: "sns_delete_topic", Permission: "sns:DeleteTopic",
			Description: "Delete SNS topics (cleanup)", Required: false, Status: "ok",
		}
		fakeARN := fmt.Sprintf("arn:aws:sns:%s:000000000000:senda-validate-perm-check", awsCfg.Region)
		_, err := snsClient.DeleteTopic(gCtx, &sns.DeleteTopicInput{
			TopicArn: aws.String(fakeARN),
		})
		if err != nil && IsAccessDenied(err) {
			c.Status = "denied"
		}
		setCheck(7, c)
		return nil
	})

	_ = g.Wait() // individual goroutines never return errors; they set status instead

	return &ValidationResult{Valid: valid, Checks: checks}, nil
}
