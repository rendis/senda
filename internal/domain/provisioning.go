package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProvisioningStepStatus represents the state of a single provisioning step.
type ProvisioningStepStatus string

const (
	ProvisionStepPending   ProvisioningStepStatus = "pending"
	ProvisionStepCompleted ProvisioningStepStatus = "completed"
	ProvisionStepFailed    ProvisioningStepStatus = "failed"
)

// ProvisionStepName identifies a provisioning step across the repo and provisioner layers.
type ProvisionStepName = string

const (
	StepCreateConfigurationSet ProvisionStepName = "create_configuration_set"
	StepCreateSNSTopic         ProvisionStepName = "create_sns_topic"
	StepCreateEventDestination ProvisionStepName = "create_event_destination"
	StepSubscribeWebhook       ProvisionStepName = "subscribe_webhook"
	StepSaveConfiguration      ProvisionStepName = "save_configuration"
	StepVerifySubscription     ProvisionStepName = "verify_subscription"

	// Deprovision steps (executed on adapter soft-delete, reverse order of provisioning).
	StepDeprovUnsubscribeWebhook       ProvisionStepName = "deprov_unsubscribe_webhook"
	StepDeprovDeleteEventDestination   ProvisionStepName = "deprov_delete_event_destination"
	StepDeprovDeleteSNSTopic           ProvisionStepName = "deprov_delete_sns_topic"
	StepDeprovDeleteConfigurationSet   ProvisionStepName = "deprov_delete_configuration_set"
)

// ProvisionStepDefs is the ordered list of SES provisioning steps.
var ProvisionStepDefs = []struct {
	Name  ProvisionStepName
	Order int
}{
	{StepCreateConfigurationSet, 1},
	{StepCreateSNSTopic, 2},
	{StepCreateEventDestination, 3},
	{StepSubscribeWebhook, 4},
	{StepSaveConfiguration, 5},
	{StepVerifySubscription, 6},
}

// DeprovisionStepDefs is the ordered list of SES deprovision steps.
var DeprovisionStepDefs = []struct {
	Name  ProvisionStepName
	Order int
}{
	{StepDeprovUnsubscribeWebhook, 10},
	{StepDeprovDeleteEventDestination, 11},
	{StepDeprovDeleteSNSTopic, 12},
	{StepDeprovDeleteConfigurationSet, 13},
}

// AdapterProvisioningStep tracks the state of a single provisioning step for an adapter.
type AdapterProvisioningStep struct {
	ID           uuid.UUID
	AdapterID    uuid.UUID
	StepName     string
	StepOrder    int
	Status       ProvisioningStepStatus
	ResourceName *string
	ResourceARN  *string
	ErrorMessage *string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
