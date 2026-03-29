//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	sesv1 "github.com/aws/aws-sdk-go-v2/service/ses"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/senda-app/senda/internal/adapter/crypto"
	"github.com/senda-app/senda/internal/adapter/postgres"
	sesadapter "github.com/senda-app/senda/internal/adapter/ses"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

func TestAWS01_SESAdapterEndpointURLAndTrackingProvisioner(t *testing.T) {
	if os.Getenv("SENDA_E2E_AWS") != "1" {
		t.Skip("AWS-sim suite disabled; set SENDA_E2E_AWS=1 to run")
	}

	EnsureSetup(t)
	endpoint := ensureLocalStack(t)
	replay := startReplayHarness(t)
	t.Cleanup(replay.Close)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	adapterName := uniqueName("aws-ses")
	adapterID := mustCreateAWSSESAdapter(t, client, endpoint, adapterName)

	cfg := mustDecryptSESAdapterConfig(t, adapterID)
	require.Equal(t, endpoint, cfg.EndpointURL, "endpoint_url should persist in adapter config")
	require.Equal(t, defaultAWSRegion, cfg.Region)

	// Smoke: call SES directly against LocalStack using the decrypted config.
	sender, err := sesadapter.NewAdapterFromConfig(context.Background(), cfg)
	require.NoError(t, err)

	sesClient, err := newLocalStackSESV1Client(context.Background(), endpoint)
	require.NoError(t, err)
	_, err = sesClient.VerifyEmailIdentity(context.Background(), &sesv1.VerifyEmailIdentityInput{
		EmailAddress: aws.String(TestFromEmail),
	})
	if err != nil {
		// The identity may already exist from a previous run; ignore duplicate-like errors.
		t.Logf("create email identity: %v", err)
	}

	providerID, err := sender.Send(context.Background(), &port.OutgoingEmail{
		From:       port.EmailAddress{Name: TestFromName, Address: TestFromEmail},
		To:         port.EmailAddress{Address: "localstack-smoke@test.example.com"},
		Subject:    "LocalStack SES smoke",
		BodyText:   "hello from localstack",
		TrackingID: "localstack-smoke-" + uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, providerID)

	// Tracking provisioner should use the adapter's endpoint_url and the replay base URL.
	pool := mustDBPool(t)
	aesCrypto, err := crypto.NewAESCrypto(DefaultMasterKey)
	require.NoError(t, err)

	provisioner := sesadapter.NewTrackingProvisioner(
		postgres.NewAdapterRepo(pool),
		aesCrypto,
		replay.server.URL,
		nil,
	)

	result, err := provisioner.Provision(context.Background(), uuid.MustParse(adapterID))
	require.NoError(t, err)
	require.NotEmpty(t, result.ConfigSetName)
	require.NotEmpty(t, result.TopicARN)
	require.NotEmpty(t, result.SubscriptionARN)
	require.Equal(t, replay.server.URL+"/api/v1/webhooks/ses/inbound", result.WebhookURL)

	cfgAfter := mustDecryptSESAdapterConfig(t, adapterID)
	require.Equal(t, result.ConfigSetName, cfgAfter.ConfigurationSetName, "provisioner should persist configuration_set_name")
}

func TestAWS02_SNSCaptureReplayHTTPDeliveryBounceComplaint(t *testing.T) {
	if os.Getenv("SENDA_E2E_AWS") != "1" {
		t.Skip("AWS-sim suite disabled; set SENDA_E2E_AWS=1 to run")
	}

	EnsureSetup(t)
	endpoint := ensureLocalStack(t)
	replay := startReplayHarness(t)
	t.Cleanup(replay.Close)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)
	mailpit.ClearMessages()

	apiKey := createAPIKey(t, client, "aws-sns-replay")
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKey)

	topicArn, queueURL, _, _ := createTopicQueueSubscription(t, endpoint, "aws-sns-replay")

	cases := []struct {
		name           string
		notification   string
		expectedStatus string
		shouldSuppress bool
		suppressTable  string
	}{
		{name: "delivery", notification: "Delivery", expectedStatus: string(domain.StatusDelivered)},
		{name: "bounce", notification: "Bounce", expectedStatus: string(domain.StatusBounced), shouldSuppress: true, suppressTable: "global"},
		{name: "complaint", notification: "Complaint", expectedStatus: string(domain.StatusComplained), shouldSuppress: true, suppressTable: "workspace"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recipient := fmt.Sprintf("%s-%s@test.example.com", tc.name, strings.ReplaceAll(uuid.NewString(), "-", "")[:8])

			resp := sendClient.Post("/api/v1/send", SendRequest{
				Ref: sendRef(),
				To:  []string{recipient},
				Variables: map[string]interface{}{
					"first_name":   "AWS",
					"company_name": "LocalStack",
				},
			})
			defer resp.Body.Close()
			RequireStatus(t, resp, http.StatusAccepted)

			var sendBody struct {
				TrackingIDs []struct {
					TrackingID string `json:"tracking_id"`
				} `json:"tracking_ids"`
			}
			ParseJSONResponse(t, resp, &sendBody)
			require.Len(t, sendBody.TrackingIDs, 1)
			trackingID := sendBody.TrackingIDs[0].TrackingID
			providerMsgID := smtpProviderMessageID(trackingID)

			// Poll status through the management-plane client. The data-plane API key used
			// for POST /send is not authorized to query management email status endpoints.
			// We must observe the row reach `sent` before replaying SNS events so the
			// provider_message_id correlation is already persisted.
			client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

			// Confirm the SMTP/Mailpit path still works before replaying AWS events.
			mailpit.WaitForMessages(1, 10*time.Second)
			mailpit.AssertMessageExists(recipient)

			notification := buildSESNotification(t, tc.notification, providerMsgID, recipient, time.Now().UTC())
			publishSESNotification(t, endpoint, topicArn, notification)

			captured := receiveSQSMessage(t, endpoint, queueURL, 20*time.Second)
			require.NotEmpty(t, captured)

			var envelope snsEnvelope
			require.NoError(t, json.Unmarshal([]byte(captured), &envelope))
			require.JSONEq(t, string(notification), envelope.Message)
			signed := signSNSMessage(t, envelope, replay.certURL, replay.privateKey)

			replayResp, err := replay.POST("/api/v1/webhooks/ses/inbound", signed)
			require.NoError(t, err)
			defer replayResp.Body.Close()
			RequireStatus(t, replayResp, http.StatusOK)

			client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, tc.expectedStatus, 45*time.Second)

			if tc.shouldSuppress {
				assertSuppressionExists(t, tc.suppressTable, recipient)
			}
		})
	}
}

func mustCreateAWSSESAdapter(t *testing.T, client *TestClient, endpoint, name string) string {
	t.Helper()

	resp := client.Post(wsPath()+"/adapters", AdapterRequest{
		Name:        name,
		AdapterType: AdapterType,
		Config: map[string]interface{}{
			"region":                 defaultAWSRegion,
			"access_key_id":          defaultAWSAccessKeyID,
			"secret_access_key":      defaultAWSSecretAccessKey,
			"endpoint_url":           endpoint,
			"configuration_set_name": "",
		},
		RateLimitPerSecond: 100,
	})
	defer resp.Body.Close()

	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID)
	return body.ID
}

func mustDecryptSESAdapterConfig(t *testing.T, adapterID string) sesadapter.Config {
	t.Helper()

	conn := dbConn(t)
	var encrypted []byte
	err := conn.QueryRow(context.Background(),
		"SELECT config_encrypted FROM adapters WHERE id = $1::uuid",
		adapterID,
	).Scan(&encrypted)
	require.NoError(t, err)

	aesCrypto, err := crypto.NewAESCrypto(DefaultMasterKey)
	require.NoError(t, err)

	plaintext, err := aesCrypto.Decrypt(encrypted)
	require.NoError(t, err)

	var cfg sesadapter.Config
	require.NoError(t, json.Unmarshal(plaintext, &cfg))
	return cfg
}

func publishSESNotification(t *testing.T, endpoint, topicArn string, notification []byte) []byte {
	t.Helper()

	snsClient, err := newLocalStackSNSClient(context.Background(), endpoint)
	require.NoError(t, err)

	out, err := snsClient.Publish(context.Background(), &awssns.PublishInput{
		TopicArn: aws.String(topicArn),
		Message:  aws.String(string(notification)),
	})
	require.NoError(t, err)
	require.NotNil(t, out.MessageId)

	return notification
}

func assertSuppressionExists(t *testing.T, table, email string) {
	t.Helper()

	conn := dbConn(t)
	var count int
	var query string
	switch table {
	case "global":
		query = "SELECT COUNT(1) FROM suppression_global WHERE email = $1 AND removed_at IS NULL"
	case "workspace":
		query = "SELECT COUNT(1) FROM suppression_workspace WHERE email = $1 AND removed_at IS NULL"
	default:
		t.Fatalf("unsupported suppression table %q", table)
	}
	require.NoError(t, conn.QueryRow(context.Background(), query, email).Scan(&count))
	require.Greater(t, count, 0, "expected suppression entry for %s", email)
}

func mustDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("SENDA_DATABASE_URL")
	require.NotEmpty(t, dbURL)
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}
