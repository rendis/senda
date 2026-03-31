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
