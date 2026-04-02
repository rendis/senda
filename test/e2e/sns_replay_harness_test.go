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
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/adapter/postgres"
	snsauth "github.com/rendis/senda/internal/adapter/sns"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/internal/teststack/awssim"
)

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
	MessageID        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	Subject          string `json:"Subject,omitempty"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
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

	body, err := awssim.BuildSESNotification(notificationType, providerMessageID, recipient, at)
	if err != nil {
		t.Fatalf("build ses notification: %v", err)
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
		b.WriteString(msg.MessageID)
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
