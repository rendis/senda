package handler

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// AdapterSetupHandler serves tracking setup guides and auto-provisioning for adapters.
type AdapterSetupHandler struct {
	store       port.AdapterStore
	tsStore     port.TenantStore
	wsStore     port.WorkspaceStore
	webhookURL  string // base URL for the SES inbound webhook
	provisioner *sesadapter.TrackingProvisioner
}

// NewAdapterSetupHandler creates a new AdapterSetupHandler.
func NewAdapterSetupHandler(as port.AdapterStore, ts port.TenantStore, ws port.WorkspaceStore, webhookURL string, provisioner *sesadapter.TrackingProvisioner) *AdapterSetupHandler {
	return &AdapterSetupHandler{store: as, tsStore: ts, wsStore: ws, webhookURL: webhookURL, provisioner: provisioner}
}

// SetupGuide handles GET /adapters/:id/setup-guide (workspace scope).
func (h *AdapterSetupHandler) SetupGuide(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.setupGuide(c, &ws.ID)
}

// SetupGuideGlobal handles GET /global/adapters/:id/setup-guide.
func (h *AdapterSetupHandler) SetupGuideGlobal(c *echo.Context) error {
	return h.setupGuide(c, nil)
}

func (h *AdapterSetupHandler) setupGuide(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	adapter, err := h.store.GetByID(c.Request().Context(), adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	switch adapter.AdapterType {
	case domain.AdapterTypeSES:
		return c.JSON(http.StatusOK, h.buildSESGuide(adapter))
	case domain.AdapterTypeGmail:
		return c.JSON(http.StatusOK, buildGmailGuide(adapter))
	default:
		return c.JSON(http.StatusOK, map[string]any{
			"adapter_type":      string(adapter.AdapterType),
			"adapter_id":        adapter.ID.String(),
			"tracking_supported": false,
			"message":           "This adapter type does not support event tracking.",
		})
	}
}

func (h *AdapterSetupHandler) buildSESGuide(adapter *domain.Adapter) map[string]any {
	shortID := adapter.ID.String()[:8]
	configSetName := fmt.Sprintf("senda-%s", shortID)
	topicName := fmt.Sprintf("senda-ses-events-%s", shortID)
	webhookEndpoint := h.webhookURL + "/api/v1/webhooks/ses/inbound"

	return map[string]any{
		"adapter_type":       string(adapter.AdapterType),
		"adapter_id":         adapter.ID.String(),
		"tracking_supported": true,
		"webhook_url":        webhookEndpoint,
		"steps": []map[string]any{
			{
				"step":                1,
				"title":               "Create Configuration Set in SES",
				"description":         "A Configuration Set tells SES to emit events (delivery, bounce, complaint) for emails sent through it.",
				"aws_cli":             fmt.Sprintf("aws sesv2 create-configuration-set --configuration-set-name %s", configSetName),
				"required_permission": "ses:CreateConfigurationSet",
			},
			{
				"step":                2,
				"title":               "Create SNS Topic",
				"description":         "An SNS topic acts as the message bus between SES and Senda.",
				"aws_cli":             fmt.Sprintf("aws sns create-topic --name %s", topicName),
				"required_permission": "sns:CreateTopic",
				"note":                "Save the TopicArn from the output — you'll need it in the next steps.",
			},
			{
				"step":                3,
				"title":               "Configure Event Destination",
				"description":         "Link the Configuration Set to the SNS Topic so SES publishes events there.",
				"aws_cli":             fmt.Sprintf("aws sesv2 create-configuration-set-event-destination --configuration-set-name %s --event-destination-name senda-events --event-destination '{\"Enabled\":true,\"MatchingEventTypes\":[\"SEND\",\"DELIVERY\",\"BOUNCE\",\"COMPLAINT\"],\"SnsDestination\":{\"TopicArn\":\"<YOUR_TOPIC_ARN>\"}}'", configSetName),
				"required_permission": "ses:CreateConfigurationSetEventDestination",
			},
			{
				"step":                4,
				"title":               "Subscribe Senda webhook to SNS Topic",
				"description":         "Create an HTTPS subscription so SNS forwards events to Senda. Senda auto-confirms the subscription.",
				"aws_cli":             fmt.Sprintf("aws sns subscribe --topic-arn <YOUR_TOPIC_ARN> --protocol https --endpoint %s", webhookEndpoint),
				"required_permission": "sns:Subscribe",
			},
			{
				"step":        5,
				"title":       "Register Configuration Set in Senda",
				"description": "Update the adapter with the Configuration Set name so Senda includes it when sending emails.",
				"api_call":    fmt.Sprintf("PATCH /adapters/%s with body: {\"configuration_set_name\": \"%s\"}", adapter.ID.String(), configSetName),
			},
		},
		"iam_policy": map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Effect":   "Allow",
					"Action":   []string{"ses:SendEmail", "ses:SendRawEmail", "ses:ListEmailIdentities"},
					"Resource": "*",
				},
				{
					"Effect":   "Allow",
					"Action":   []string{"ses:CreateConfigurationSet", "ses:CreateConfigurationSetEventDestination"},
					"Resource": "*",
				},
				{
					"Effect":   "Allow",
					"Action":   []string{"sns:CreateTopic", "sns:Subscribe"},
					"Resource": "*",
				},
			},
		},
	}
}

// AutoProvision handles POST /adapters/:id/auto-provision-tracking (workspace scope).
func (h *AdapterSetupHandler) AutoProvision(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.autoProvision(c, &ws.ID)
}

// AutoProvisionGlobal handles POST /global/adapters/:id/auto-provision-tracking.
func (h *AdapterSetupHandler) AutoProvisionGlobal(c *echo.Context) error {
	return h.autoProvision(c, nil)
}

func (h *AdapterSetupHandler) autoProvision(c *echo.Context, workspaceID *uuid.UUID) error {
	if h.provisioner == nil {
		return response.WriteError(c, http.StatusNotImplemented, "NOT_IMPLEMENTED",
			"auto-provisioning is not available (tracking base URL not configured)")
	}

	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	adapter, err := h.store.GetByID(c.Request().Context(), adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	result, err := h.provisioner.Provision(c.Request().Context(), adapterID)
	if err != nil {
		if sesadapter.IsAccessDenied(err) {
			return c.JSON(http.StatusForbidden, map[string]any{
				"error":   "INSUFFICIENT_PERMISSIONS",
				"message": "The adapter's AWS credentials lack permissions for auto-provisioning. Use the setup guide for manual configuration.",
				"result":  result,
			})
		}
		// Return partial result with error info.
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"error":   "PROVISIONING_FAILED",
			"message": err.Error(),
			"result":  result,
		})
	}

	return c.JSON(http.StatusOK, result)
}

func buildGmailGuide(adapter *domain.Adapter) map[string]any {
	return map[string]any{
		"adapter_type":       string(adapter.AdapterType),
		"adapter_id":         adapter.ID.String(),
		"tracking_supported": false,
		"message":            "Gmail API does not support delivery/bounce webhooks. Emails sent via Gmail will show as 'sent' without further status updates. For full tracking (delivery, bounce, complaint, open), use an SES adapter.",
	}
}
