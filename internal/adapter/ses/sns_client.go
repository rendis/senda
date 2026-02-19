package ses

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSAPI abstracts the SNS API for testability.
type SNSAPI interface {
	CreateTopic(ctx context.Context, params *sns.CreateTopicInput, optFns ...func(*sns.Options)) (*sns.CreateTopicOutput, error)
	Subscribe(ctx context.Context, params *sns.SubscribeInput, optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)
}
