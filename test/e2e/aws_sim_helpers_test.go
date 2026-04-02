//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
)

func newAWSSimConfig(ctx context.Context, endpoint string) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(defaultAWSRegion),
		awsconfig.WithBaseEndpoint(endpoint),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(defaultAWSAccessKeyID, defaultAWSSecretAccessKey, ""),
		),
	)
}

func newAWSSimSESV2Client(ctx context.Context, endpoint string) (*sesv2.Client, error) {
	cfg, err := newAWSSimConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

func newAWSSimSNSClient(ctx context.Context, endpoint string) (*sns.Client, error) {
	cfg, err := newAWSSimConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
}

func mustCreateAWSSimEmailIdentity(t *testing.T, endpoint, identity string) {
	t.Helper()

	client, err := newAWSSimSESV2Client(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("create aws-sim ses client: %v", err)
	}

	_, err = client.CreateEmailIdentity(context.Background(), &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(identity),
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("create aws-sim identity %q: %v", identity, err)
	}
}
