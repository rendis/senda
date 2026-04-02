package awssim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

const (
	DefaultRegion          = "us-east-1"
	DefaultAccessKeyID     = "test"
	DefaultSecretAccessKey = "test"
)

var (
	createEventDestinationPath = regexp.MustCompile(`^/v2/email/configuration-sets/([^/]+)/event-destinations$`)
	deleteEventDestinationPath = regexp.MustCompile(`^/v2/email/configuration-sets/([^/]+)/event-destinations/([^/]+)$`)
)

type Config struct {
	BackendBaseURL  string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Publisher       Publisher
	Subscriptions   SubscriptionResolver
	Deliverer       NotificationDeliverer
}

type EventDestinationRecord struct {
	ConfigurationSetName string    `json:"configuration_set_name"`
	EventDestinationName string    `json:"event_destination_name"`
	TopicARN             string    `json:"topic_arn"`
	MatchingEventTypes   []string  `json:"matching_event_types,omitempty"`
	Enabled              bool      `json:"enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type MessageRecord struct {
	ProviderMessageID    string    `json:"provider_message_id"`
	ConfigurationSetName string    `json:"configuration_set_name,omitempty"`
	Recipients           []string  `json:"recipients,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type State struct {
	EventDestinations []EventDestinationRecord `json:"event_destinations"`
	Messages          map[string]MessageRecord `json:"messages"`
}

type emitEventRequest struct {
	NotificationType  string    `json:"notification_type"`
	ProviderMessageID string    `json:"provider_message_id"`
	Recipient         string    `json:"recipient,omitempty"`
	Timestamp         time.Time `json:"timestamp,omitempty"`
}

type createEventDestinationRequest struct {
	EventDestinationName string `json:"EventDestinationName"`
	EventDestination     struct {
		Enabled            bool     `json:"Enabled"`
		MatchingEventTypes []string `json:"MatchingEventTypes"`
		SnsDestination     struct {
			TopicARN string `json:"TopicArn"`
		} `json:"SnsDestination"`
	} `json:"EventDestination"`
}

type sendEmailRequest struct {
	ConfigurationSetName string `json:"ConfigurationSetName"`
	Destination          struct {
		ToAddresses  []string `json:"ToAddresses"`
		CcAddresses  []string `json:"CcAddresses"`
		BccAddresses []string `json:"BccAddresses"`
	} `json:"Destination"`
}

type sendEmailResponse struct {
	MessageID string `json:"MessageId"`
}

type sesNotification struct {
	NotificationType string `json:"notificationType"`
	Mail             struct {
		MessageID string `json:"messageId"`
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

type Publisher interface {
	Publish(ctx context.Context, topicARN, message string) error
}

type SubscriptionResolver interface {
	HTTPSubscriptions(ctx context.Context, topicARN string) ([]string, error)
}

type NotificationDeliverer interface {
	Deliver(ctx context.Context, endpoint string, envelope []byte) error
}

type Bridge struct {
	backendURL    *url.URL
	proxy         *httputil.ReverseProxy
	publisher     Publisher
	subscriptions SubscriptionResolver
	deliverer     NotificationDeliverer

	mu                sync.RWMutex
	eventDestinations map[string]EventDestinationRecord
	messages          map[string]MessageRecord
}

func NewBridge(cfg Config) (*Bridge, error) {
	if strings.TrimSpace(cfg.BackendBaseURL) == "" {
		return nil, fmt.Errorf("backend base url is required")
	}
	backendURL, err := url.Parse(cfg.BackendBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse backend url: %w", err)
	}

	bridge := &Bridge{
		backendURL:        backendURL,
		proxy:             httputil.NewSingleHostReverseProxy(backendURL),
		eventDestinations: make(map[string]EventDestinationRecord),
		messages:          make(map[string]MessageRecord),
	}
	bridge.proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	region := defaultIfEmpty(cfg.Region, DefaultRegion)
	accessKeyID := defaultIfEmpty(cfg.AccessKeyID, DefaultAccessKeyID)
	secretAccessKey := defaultIfEmpty(cfg.SecretAccessKey, DefaultSecretAccessKey)

	if cfg.Publisher != nil {
		bridge.publisher = cfg.Publisher
	} else {
		publisher, err := newSNSPublisher(context.Background(), cfg.BackendBaseURL, region, accessKeyID, secretAccessKey)
		if err != nil {
			return nil, err
		}
		bridge.publisher = publisher
	}

	if cfg.Subscriptions != nil {
		bridge.subscriptions = cfg.Subscriptions
	} else {
		resolver, err := newSNSSubscriptionResolver(context.Background(), cfg.BackendBaseURL, region, accessKeyID, secretAccessKey)
		if err != nil {
			return nil, err
		}
		bridge.subscriptions = resolver
	}

	if cfg.Deliverer != nil {
		bridge.deliverer = cfg.Deliverer
	} else {
		bridge.deliverer = &httpNotificationDeliverer{client: &http.Client{Timeout: 10 * time.Second}}
	}

	return bridge, nil
}

func (b *Bridge) Handler() http.Handler {
	return http.HandlerFunc(b.serveHTTP)
}

func (b *Bridge) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/_aws-sim/health":
		b.handleHealth(w, r)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/_aws-sim/state":
		b.handleState(w)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/_aws-sim/control/ses-events":
		b.handleEmitSESEvent(w, r)
		return
	case r.Method == http.MethodPost && createEventDestinationPath.MatchString(r.URL.Path):
		b.handleCreateEventDestination(w, r)
		return
	case r.Method == http.MethodDelete && deleteEventDestinationPath.MatchString(r.URL.Path):
		b.handleDeleteEventDestination(w, r)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/v2/email/outbound-emails":
		b.handleSendEmail(w, r)
		return
	default:
		b.proxy.ServeHTTP(w, r)
	}
}

func (b *Bridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, b.backendURL.String()+"/_ministack/health", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (b *Bridge) handleState(w http.ResponseWriter) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := State{
		EventDestinations: make([]EventDestinationRecord, 0, len(b.eventDestinations)),
		Messages:          make(map[string]MessageRecord, len(b.messages)),
	}
	for _, record := range b.eventDestinations {
		state.EventDestinations = append(state.EventDestinations, record)
	}
	slices.SortFunc(state.EventDestinations, func(a, c EventDestinationRecord) int {
		return strings.Compare(a.ConfigurationSetName+a.EventDestinationName, c.ConfigurationSetName+c.EventDestinationName)
	})
	for id, record := range b.messages {
		state.Messages[id] = record
	}

	writeJSON(w, http.StatusOK, state)
}

func (b *Bridge) handleCreateEventDestination(w http.ResponseWriter, r *http.Request) {
	matches := createEventDestinationPath.FindStringSubmatch(r.URL.Path)
	if len(matches) != 2 {
		http.Error(w, "invalid event destination path", http.StatusBadRequest)
		return
	}
	configSetName := matches[1]

	var req createEventDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	record := EventDestinationRecord{
		ConfigurationSetName: configSetName,
		EventDestinationName: req.EventDestinationName,
		TopicARN:             req.EventDestination.SnsDestination.TopicARN,
		MatchingEventTypes:   append([]string(nil), req.EventDestination.MatchingEventTypes...),
		Enabled:              req.EventDestination.Enabled,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	b.mu.Lock()
	if existing, ok := b.eventDestinations[eventDestinationKey(configSetName, req.EventDestinationName)]; ok {
		record.CreatedAt = existing.CreatedAt
	}
	b.eventDestinations[eventDestinationKey(configSetName, req.EventDestinationName)] = record
	b.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{})
}

func (b *Bridge) handleDeleteEventDestination(w http.ResponseWriter, r *http.Request) {
	matches := deleteEventDestinationPath.FindStringSubmatch(r.URL.Path)
	if len(matches) != 3 {
		http.Error(w, "invalid event destination path", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	delete(b.eventDestinations, eventDestinationKey(matches[1], matches[2]))
	b.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{})
}

func (b *Bridge) handleSendEmail(w http.ResponseWriter, r *http.Request) {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	var sendReq sendEmailRequest
	_ = json.Unmarshal(requestBody, &sendReq)

	backendReq, err := http.NewRequestWithContext(r.Context(), r.Method, b.backendURL.String()+r.URL.RequestURI(), bytes.NewReader(requestBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	copyHeaders(backendReq.Header, r.Header)

	resp, err := http.DefaultClient.Do(backendReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var sendResp sendEmailResponse
		if json.Unmarshal(responseBody, &sendResp) == nil && strings.TrimSpace(sendResp.MessageID) != "" {
			b.mu.Lock()
			b.messages[sendResp.MessageID] = MessageRecord{
				ProviderMessageID:    sendResp.MessageID,
				ConfigurationSetName: sendReq.ConfigurationSetName,
				Recipients:           collectRecipients(sendReq),
				CreatedAt:            time.Now().UTC(),
			}
			b.mu.Unlock()
		}
	}

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(responseBody)
}

func (b *Bridge) handleEmitSESEvent(w http.ResponseWriter, r *http.Request) {
	var req emitEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.NotificationType == "" || req.ProviderMessageID == "" {
		http.Error(w, "notification_type and provider_message_id are required", http.StatusBadRequest)
		return
	}

	b.mu.RLock()
	msg, ok := b.messages[req.ProviderMessageID]
	if !ok {
		b.mu.RUnlock()
		http.Error(w, "provider message id not found", http.StatusNotFound)
		return
	}
	topicARN, ok := b.resolveTopicARNLocked(msg.ConfigurationSetName, req.NotificationType)
	b.mu.RUnlock()
	if !ok {
		http.Error(w, "topic arn not found for configuration set", http.StatusNotFound)
		return
	}

	recipient := req.Recipient
	if recipient == "" && len(msg.Recipients) > 0 {
		recipient = msg.Recipients[0]
	}
	if recipient == "" {
		http.Error(w, "recipient is required", http.StatusBadRequest)
		return
	}

	at := req.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	notification, err := BuildSESNotification(req.NotificationType, req.ProviderMessageID, recipient, at)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := b.publisher.Publish(r.Context(), topicARN, string(notification)); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	endpoints, err := b.subscriptions.HTTPSubscriptions(r.Context(), topicARN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(endpoints) == 0 {
		http.Error(w, "no sns webhook subscriptions found", http.StatusBadGateway)
		return
	}

	envelope, err := buildSNSEnvelope(topicARN, string(notification), at)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for _, endpoint := range endpoints {
		if err := b.deliverer.Deliver(r.Context(), endpoint, envelope); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"topic_arn":           topicARN,
		"notification":        req.NotificationType,
		"provider_message_id": req.ProviderMessageID,
	})
}

func (b *Bridge) resolveTopicARNLocked(configurationSetName, notificationType string) (string, bool) {
	want := strings.ToUpper(notificationType)
	for _, record := range b.eventDestinations {
		if record.ConfigurationSetName != configurationSetName || !record.Enabled {
			continue
		}
		if len(record.MatchingEventTypes) == 0 || slices.Contains(record.MatchingEventTypes, want) {
			return record.TopicARN, true
		}
	}
	return "", false
}

func collectRecipients(req sendEmailRequest) []string {
	recipients := make([]string, 0, len(req.Destination.ToAddresses)+len(req.Destination.CcAddresses)+len(req.Destination.BccAddresses))
	recipients = append(recipients, req.Destination.ToAddresses...)
	recipients = append(recipients, req.Destination.CcAddresses...)
	recipients = append(recipients, req.Destination.BccAddresses...)
	return recipients
}

func eventDestinationKey(configurationSetName, eventDestinationName string) string {
	return configurationSetName + "::" + eventDestinationName
}

func BuildSESNotification(notificationType, providerMessageID, recipient string, at time.Time) ([]byte, error) {
	notif := sesNotification{
		NotificationType: notificationType,
	}
	notif.Mail.MessageID = providerMessageID

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
			FeedbackID: fmt.Sprintf("feedback-%d", at.UnixNano()),
			Timestamp:  at.UTC().Format(time.RFC3339),
		}
	default:
		return nil, fmt.Errorf("unsupported notification type %q", notificationType)
	}

	return json.Marshal(notif)
}

type snsPublisher struct {
	client *sns.Client
}

type snsSubscriptionResolver struct {
	client *sns.Client
}

type httpNotificationDeliverer struct {
	client *http.Client
}

func newSNSPublisher(ctx context.Context, endpoint, region, accessKeyID, secretAccessKey string) (Publisher, error) {
	client, err := newSNSClient(ctx, endpoint, region, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}
	return &snsPublisher{client: client}, nil
}

func newSNSSubscriptionResolver(ctx context.Context, endpoint, region, accessKeyID, secretAccessKey string) (SubscriptionResolver, error) {
	client, err := newSNSClient(ctx, endpoint, region, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}
	return &snsSubscriptionResolver{client: client}, nil
}

func newSNSClient(ctx context.Context, endpoint, region, accessKeyID, secretAccessKey string) (*sns.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithBaseEndpoint(endpoint),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

func (p *snsPublisher) Publish(ctx context.Context, topicARN, message string) error {
	_, err := p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(topicARN),
		Message:  aws.String(message),
	})
	return err
}

func (r *snsSubscriptionResolver) HTTPSubscriptions(ctx context.Context, topicARN string) ([]string, error) {
	out, err := r.client.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
		TopicArn: aws.String(topicARN),
	})
	if err != nil {
		return nil, err
	}
	endpoints := make([]string, 0, len(out.Subscriptions))
	for _, sub := range out.Subscriptions {
		protocol := strings.ToLower(aws.ToString(sub.Protocol))
		if protocol != "http" && protocol != "https" {
			continue
		}
		endpoint := strings.TrimSpace(aws.ToString(sub.Endpoint))
		if endpoint == "" {
			continue
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

func (d *httpNotificationDeliverer) Deliver(ctx context.Context, endpoint string, envelope []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(envelope))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver webhook: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("deliver webhook: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func buildSNSEnvelope(topicARN, message string, at time.Time) ([]byte, error) {
	envelope := map[string]any{
		"Type":             "Notification",
		"MessageId":        fmt.Sprintf("aws-sim-%d", at.UnixNano()),
		"TopicArn":         topicARN,
		"Message":          message,
		"Timestamp":        at.UTC().Format(time.RFC3339),
		"SignatureVersion": "1",
		"Signature":        "FAKE",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/SimpleNotificationService.pem",
	}
	return json.Marshal(envelope)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
