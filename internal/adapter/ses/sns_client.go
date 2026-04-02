package ses

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSAPI abstracts the SNS API for testability.
type SNSAPI interface {
	CreateTopic(ctx context.Context, params *sns.CreateTopicInput, optFns ...func(*sns.Options)) (*sns.CreateTopicOutput, error)
	Subscribe(ctx context.Context, params *sns.SubscribeInput, optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)
	GetSubscriptionAttributes(ctx context.Context, params *sns.GetSubscriptionAttributesInput, optFns ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error)
	ListSubscriptionsByTopic(ctx context.Context, params *sns.ListSubscriptionsByTopicInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error)
	Unsubscribe(ctx context.Context, params *sns.UnsubscribeInput, optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
	DeleteTopic(ctx context.Context, params *sns.DeleteTopicInput, optFns ...func(*sns.Options)) (*sns.DeleteTopicOutput, error)
}
