//go:build e2e

package e2e

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	sesv1 "github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/adapter/postgres"
	snsauth "github.com/rendis/senda/internal/adapter/sns"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/service"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultLocalStackImage = "localstack/localstack:4.14.0"
	localStackName         = "senda-e2e-localstack"
	localStackPort         = "4566/tcp"
)

type localStackHarness struct {
	once     sync.Once
	ctr      testcontainers.Container
	endpoint string
	err      error
}

var awsLocalStack localStackHarness

func ensureLocalStack(t *testing.T) string {
	t.Helper()
	awsLocalStack.once.Do(func() {
		awsLocalStack.ctr, awsLocalStack.endpoint, awsLocalStack.err = startLocalStackContainer(context.Background())
	})
	if awsLocalStack.err != nil {
		t.Fatalf("starting localstack: %v", awsLocalStack.err)
	}
	return awsLocalStack.endpoint
}

func startLocalStackContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        resolveLocalStackImage(),
		ExposedPorts: []string{localStackPort},
		Env: map[string]string{
			"SERVICES":              "ses,sns,sqs",
			"DEBUG":                 "0",
			"DEFAULT_REGION":        defaultAWSRegion,
			"AWS_DEFAULT_REGION":    defaultAWSRegion,
			"AWS_ACCESS_KEY_ID":     defaultAWSAccessKeyID,
			"AWS_SECRET_ACCESS_KEY": defaultAWSSecretAccessKey,
			"LS_LOG":                "warn",
			"PERSISTENCE":           "0",
		},
		Name: localStackName,
		WaitingFor: wait.ForHTTP("/_localstack/health").
			WithPort(localStackPort).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start localstack container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("localstack host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, localStackPort)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("localstack mapped port: %w", err)
	}
	return ctr, fmt.Sprintf("http://%s:%s", host, mappedPort.Port()), nil
}

func resolveLocalStackImage() string {
	if image := strings.TrimSpace(os.Getenv("SENDA_LOCALSTACK_IMAGE")); image != "" {
		return image
	}
	return defaultLocalStackImage
}

func newAWSConfig(ctx context.Context, endpoint string) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(defaultAWSRegion),
		awsconfig.WithBaseEndpoint(endpoint),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(defaultAWSAccessKeyID, defaultAWSSecretAccessKey, ""),
		),
	)
}

func newLocalStackSESClient(ctx context.Context, endpoint string) (*sesv2.Client, error) {
	cfg, err := newAWSConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sesv2.NewFromConfig(cfg), nil
}

func newLocalStackSESV1Client(ctx context.Context, endpoint string) (*sesv1.Client, error) {
	cfg, err := newAWSConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sesv1.NewFromConfig(cfg), nil
}

func newLocalStackSNSClient(ctx context.Context, endpoint string) (*sns.Client, error) {
	cfg, err := newAWSConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sns.NewFromConfig(cfg), nil
}

func newLocalStackSQSClient(ctx context.Context, endpoint string) (*sqs.Client, error) {
	cfg, err := newAWSConfig(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(cfg), nil
}

type replayHarness struct {
	server     *httptest.Server
	certServer *httptest.Server
	client     *http.Client
	pool       *pgxpool.Pool
	certPEM    []byte
	privateKey *rsa.PrivateKey
	certURL    string
}

func startReplayHarness(t *testing.T) *replayHarness {
	t.Helper()

	certPEM, privateKey := mustGenerateSNSCert(t)
	certServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(certPEM)
	}))

	verifierClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := &net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, certServer.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	dbURL := os.Getenv("SENDA_DATABASE_URL")
	if dbURL == "" {
		t.Fatal("SENDA_DATABASE_URL is empty")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("opening db pool: %v", err)
	}

	emailRepo := postgres.NewEmailRepo(pool)
	webhookRepo := postgres.NewWebhookRepo(pool)
	suppressionRepo := postgres.NewSuppressionRepo(pool)
	webhookSvc := service.NewWebhookService(webhookRepo, nil)
	eventProcessor := service.NewEventProcessor(emailRepo, emailRepo, suppressionRepo, webhookSvc, nil)

	snsVerifier := snsauth.NewVerifier(verifierClient)
	sesWebhook := handler.NewSESWebhookHandler(eventProcessor, snsVerifier, nil, nil)
	appCfg := &config.Config{
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            0,
			ShutdownTimeout: 5 * time.Second,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewTLSServer(sendahttp.NewServer(appCfg, logger, sendahttp.WithSESWebhookHandler(sesWebhook)).Echo())

	return &replayHarness{
		server:     server,
		certServer: certServer,
		client:     server.Client(),
		pool:       pool,
		certPEM:    certPEM,
		privateKey: privateKey,
		certURL:    "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-test.pem",
	}
}

func (h *replayHarness) Close() {
	if h == nil {
		return
	}
	if h.server != nil {
		h.server.Close()
	}
	if h.certServer != nil {
		h.certServer.Close()
	}
	if h.pool != nil {
		h.pool.Close()
	}
}

func (h *replayHarness) POST(urlPath string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, h.server.URL+urlPath, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.client.Do(req)
}

type snsEnvelope struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	Subject          string `json:"Subject,omitempty"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
}

type sesNotification struct {
	NotificationType string `json:"notificationType"`
	Mail             struct {
		MessageId string `json:"messageId"`
	} `json:"mail"`
	Bounce *struct {
		BounceType        string `json:"bounceType"`
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
		Timestamp string `json:"timestamp"`
	} `json:"bounce,omitempty"`
	Complaint *struct {
		ComplaintFeedbackType string `json:"complaintFeedbackType"`
		ComplainedRecipients  []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
		FeedbackID string `json:"feedbackId,omitempty"`
		Timestamp  string `json:"timestamp"`
	} `json:"complaint,omitempty"`
	Delivery *struct {
		Timestamp string `json:"timestamp"`
	} `json:"delivery,omitempty"`
}

func mustGenerateSNSCert(t *testing.T) ([]byte, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate sns key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sns.us-east-1.amazonaws.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create sns cert: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), key
}

func buildSESNotification(t *testing.T, notificationType, providerMessageID, recipient string, at time.Time) []byte {
	t.Helper()

	notif := sesNotification{
		NotificationType: notificationType,
	}
	notif.Mail.MessageId = providerMessageID

	switch notificationType {
	case "Delivery":
		notif.Delivery = &struct {
			Timestamp string `json:"timestamp"`
		}{Timestamp: at.UTC().Format(time.RFC3339)}
	case "Bounce":
		notif.Bounce = &struct {
			BounceType        string `json:"bounceType"`
			BouncedRecipients []struct {
				EmailAddress string `json:"emailAddress"`
			} `json:"bouncedRecipients"`
			Timestamp string `json:"timestamp"`
		}{
			BounceType: "Permanent",
			BouncedRecipients: []struct {
				EmailAddress string `json:"emailAddress"`
			}{{EmailAddress: recipient}},
			Timestamp: at.UTC().Format(time.RFC3339),
		}
	case "Complaint":
		notif.Complaint = &struct {
			ComplaintFeedbackType string `json:"complaintFeedbackType"`
			ComplainedRecipients  []struct {
				EmailAddress string `json:"emailAddress"`
			} `json:"complainedRecipients"`
			FeedbackID string `json:"feedbackId,omitempty"`
			Timestamp  string `json:"timestamp"`
		}{
			ComplaintFeedbackType: "abuse",
			ComplainedRecipients: []struct {
				EmailAddress string `json:"emailAddress"`
			}{{EmailAddress: recipient}},
			FeedbackID: uuid.NewString(),
			Timestamp:  at.UTC().Format(time.RFC3339),
		}
	default:
		t.Fatalf("unsupported notification type %q", notificationType)
	}

	body, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("marshal ses notification: %v", err)
	}
	return body
}

func signSNSMessage(t *testing.T, envelope snsEnvelope, certURL string, key *rsa.PrivateKey) []byte {
	t.Helper()

	signature := buildSNSStringToSign(envelope)
	h := sha1.New()
	_, _ = h.Write([]byte(signature))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, h.Sum(nil))
	if err != nil {
		t.Fatalf("sign sns message: %v", err)
	}

	envelope.SignatureVersion = "1"
	envelope.Signature = base64.StdEncoding.EncodeToString(sig)
	envelope.SigningCertURL = certURL

	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal sns envelope: %v", err)
	}
	return body
}

func buildSNSStringToSign(msg snsEnvelope) string {
	var b strings.Builder

	switch msg.Type {
	case "Notification":
		b.WriteString("Message\n")
		b.WriteString(msg.Message)
		b.WriteString("\n")
		b.WriteString("MessageId\n")
		b.WriteString(msg.MessageId)
		b.WriteString("\n")
		if msg.Subject != "" {
			b.WriteString("Subject\n")
			b.WriteString(msg.Subject)
			b.WriteString("\n")
		}
		b.WriteString("Timestamp\n")
		b.WriteString(msg.Timestamp)
		b.WriteString("\n")
		b.WriteString("TopicArn\n")
		b.WriteString(msg.TopicArn)
		b.WriteString("\n")
		b.WriteString("Type\n")
		b.WriteString(msg.Type)
		b.WriteString("\n")
	default:
		panic("unsupported sns type")
	}

	return b.String()
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
}

func smtpProviderMessageID(trackingID string) string {
	return fmt.Sprintf("<trk-%s@senda>", trackingID)
}

func createTopicQueueSubscription(t *testing.T, endpoint string, namePrefix string) (topicArn, queueURL, queueArn, subscriptionArn string) {
	t.Helper()
	ctx := context.Background()

	snsClient, err := newLocalStackSNSClient(ctx, endpoint)
	if err != nil {
		t.Fatalf("create sns client: %v", err)
	}
	sqsClient, err := newLocalStackSQSClient(ctx, endpoint)
	if err != nil {
		t.Fatalf("create sqs client: %v", err)
	}

	topicOut, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(uniqueName(namePrefix + "-topic")),
	})
	if err != nil {
		t.Fatalf("create sns topic: %v", err)
	}
	topicArn = aws.ToString(topicOut.TopicArn)

	queueOut, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(uniqueName(namePrefix + "-queue")),
	})
	if err != nil {
		t.Fatalf("create sqs queue: %v", err)
	}
	queueURL = aws.ToString(queueOut.QueueUrl)

	attrOut, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	if err != nil {
		t.Fatalf("get queue attributes: %v", err)
	}
	queueArn = attrOut.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]

	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"sns.amazonaws.com"},"Action":"sqs:SendMessage","Resource":"%s","Condition":{"ArnEquals":{"aws:SourceArn":"%s"}}}]}`, queueArn, topicArn)
	_, err = sqsClient.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		Attributes: map[string]string{
			string(sqstypes.QueueAttributeNamePolicy): policy,
		},
	})
	if err != nil {
		t.Fatalf("set queue policy: %v", err)
	}

	subOut, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn:              aws.String(topicArn),
		Protocol:              aws.String("sqs"),
		Endpoint:              aws.String(queueArn),
		ReturnSubscriptionArn: true,
	})
	if err != nil {
		t.Fatalf("subscribe queue: %v", err)
	}
	subscriptionArn = aws.ToString(subOut.SubscriptionArn)

	return topicArn, queueURL, queueArn, subscriptionArn
}

func receiveSQSMessage(t *testing.T, endpoint, queueURL string, timeout time.Duration) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sqsClient, err := newLocalStackSQSClient(ctx, endpoint)
	if err != nil {
		t.Fatalf("create sqs client: %v", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     1,
		})
		if err != nil {
			t.Fatalf("receive sqs message: %v", err)
		}
		if len(out.Messages) > 0 {
			return aws.ToString(out.Messages[0].Body)
		}
	}

	t.Fatalf("timed out waiting for SQS message")
	return ""
}

func terminateLocalStack(ctx context.Context) {
	if awsLocalStack.ctr != nil {
		_ = testcontainers.TerminateContainer(awsLocalStack.ctr)
	}
}
