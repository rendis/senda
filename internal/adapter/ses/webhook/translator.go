package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rendis/senda/internal/domain"
)

// Kind identifies the normalized outcome of translating an SNS/SES payload.
type Kind string

const (
	KindNotification             Kind = "notification"
	KindSubscriptionConfirmation Kind = "subscription_confirmation"
	KindIgnored                  Kind = "ignored"
)

// ParsedMessage is the normalized transport payload returned to HTTP handlers.
type ParsedMessage struct {
	Kind             Kind
	MessageID        string
	TopicArn         string
	Timestamp        string
	SubscribeURL     string
	NotificationType string
	Event            *domain.ProviderEvent
}

// Translator converts raw SNS/SES payloads into transport-friendly messages.
type Translator interface {
	Translate(rawBody []byte) (*ParsedMessage, error)
}

type translator struct{}

// NewTranslator creates a SES webhook translator.
func NewTranslator() Translator {
	return translator{}
}

type errorCode string

const (
	errorCodeBadRequest            errorCode = "bad_request"
	errorCodeMalformedNotification errorCode = "malformed_notification"
)

// TranslateError annotates transport decisions without leaking provider specifics into handlers.
type TranslateError struct {
	code errorCode
	err  error
}

func (e *TranslateError) Error() string {
	if e == nil {
		return ""
	}
	return e.err.Error()
}

func (e *TranslateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// IsBadRequest reports whether translation failed due to an invalid SNS envelope.
func IsBadRequest(err error) bool {
	var target *TranslateError
	return errors.As(err, &target) && target.code == errorCodeBadRequest
}

// IsMalformedNotification reports whether the SNS envelope was valid but the SES payload should be ignored.
func IsMalformedNotification(err error) bool {
	var target *TranslateError
	return errors.As(err, &target) && target.code == errorCodeMalformedNotification
}

// snsSubscribeURLHostRe validates that SubscribeURL hosts are SNS endpoints.
var snsSubscribeURLHostRe = regexp.MustCompile(`^sns\.[a-z]{2}(-[a-z]+-\d+)\.amazonaws\.com$`)

type snsMessage struct {
	Type             string `json:"Type"`
	MessageID        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
}

type sesNotification struct {
	NotificationType string        `json:"notificationType"`
	Mail             sesMail       `json:"mail"`
	Bounce           *sesBounce    `json:"bounce,omitempty"`
	Complaint        *sesComplaint `json:"complaint,omitempty"`
	Delivery         *sesDelivery  `json:"delivery,omitempty"`
}

type sesMail struct {
	MessageID string `json:"messageId"`
}

type sesBounce struct {
	BounceType        string `json:"bounceType"`
	BouncedRecipients []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"bouncedRecipients"`
	Timestamp string `json:"timestamp"`
}

type sesComplaint struct {
	ComplaintFeedbackType string `json:"complaintFeedbackType"`
	ComplainedRecipients  []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"complainedRecipients"`
	FeedbackID string `json:"feedbackId,omitempty"`
	Timestamp  string `json:"timestamp"`
}

type sesDelivery struct {
	Timestamp string `json:"timestamp"`
}

func (translator) Translate(rawBody []byte) (*ParsedMessage, error) {
	var msg snsMessage
	if err := json.Unmarshal(rawBody, &msg); err != nil {
		return nil, badRequestError("parse SNS message", err)
	}

	if !strings.HasPrefix(msg.TopicArn, "arn:aws:sns:") {
		return nil, badRequestError("invalid TopicArn", fmt.Errorf("topic arn %q must start with arn:aws:sns", msg.TopicArn))
	}

	switch msg.Type {
	case "SubscriptionConfirmation":
		if msg.SubscribeURL == "" {
			return nil, badRequestError("subscription confirmation missing SubscribeURL", errors.New("missing SubscribeURL"))
		}
		if err := validateSubscribeURL(msg.SubscribeURL); err != nil {
			return nil, badRequestError("invalid SubscribeURL", err)
		}

		return &ParsedMessage{
			Kind:         KindSubscriptionConfirmation,
			MessageID:    msg.MessageID,
			TopicArn:     msg.TopicArn,
			Timestamp:    msg.Timestamp,
			SubscribeURL: msg.SubscribeURL,
		}, nil

	case "Notification":
		return translateNotification(msg, rawBody)

	default:
		return &ParsedMessage{
			Kind:      KindIgnored,
			MessageID: msg.MessageID,
			TopicArn:  msg.TopicArn,
			Timestamp: msg.Timestamp,
		}, nil
	}
}

func translateNotification(msg snsMessage, rawBody []byte) (*ParsedMessage, error) {
	var notification sesNotification
	if err := json.Unmarshal([]byte(msg.Message), &notification); err != nil {
		return nil, &TranslateError{
			code: errorCodeMalformedNotification,
			err:  fmt.Errorf("parse SES notification: %w", err),
		}
	}

	event := mapSESToProviderEvent(&notification, rawBody)
	if event == nil {
		return &ParsedMessage{
			Kind:             KindIgnored,
			MessageID:        msg.MessageID,
			TopicArn:         msg.TopicArn,
			Timestamp:        msg.Timestamp,
			NotificationType: notification.NotificationType,
		}, nil
	}

	return &ParsedMessage{
		Kind:             KindNotification,
		MessageID:        msg.MessageID,
		TopicArn:         msg.TopicArn,
		Timestamp:        msg.Timestamp,
		NotificationType: notification.NotificationType,
		Event:            event,
	}, nil
}

func mapSESToProviderEvent(n *sesNotification, rawBody []byte) *domain.ProviderEvent {
	event := &domain.ProviderEvent{
		ProviderMessageID: n.Mail.MessageID,
		RawPayload:        rawBody,
	}

	switch n.NotificationType {
	case "Delivery":
		event.Type = domain.EventDelivered
		if n.Delivery != nil {
			event.Timestamp = parseSESTimestamp(n.Delivery.Timestamp)
		}

	case "Bounce":
		event.Type = domain.EventBounced
		if n.Bounce != nil {
			event.Timestamp = parseSESTimestamp(n.Bounce.Timestamp)

			bounceType := "soft"
			if n.Bounce.BounceType == "Permanent" {
				bounceType = "hard"
			}

			recipients := make([]string, 0, len(n.Bounce.BouncedRecipients))
			for _, recipient := range n.Bounce.BouncedRecipients {
				recipients = append(recipients, recipient.EmailAddress)
			}

			event.BounceDetail = &domain.BounceDetail{
				BounceType: bounceType,
				Recipients: recipients,
			}
		}

	case "Complaint":
		event.Type = domain.EventComplained
		if n.Complaint != nil {
			event.Timestamp = parseSESTimestamp(n.Complaint.Timestamp)

			recipients := make([]string, 0, len(n.Complaint.ComplainedRecipients))
			for _, recipient := range n.Complaint.ComplainedRecipients {
				recipients = append(recipients, recipient.EmailAddress)
			}

			event.ComplaintDetail = &domain.ComplaintDetail{
				ComplaintType: n.Complaint.ComplaintFeedbackType,
				FeedbackID:    n.Complaint.FeedbackID,
				Recipients:    recipients,
			}
		}

	case "Send":
		return nil

	default:
		return nil
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	return event
}

func parseSESTimestamp(ts string) time.Time {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return parsed
	}

	parsed, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func validateSubscribeURL(subscribeURL string) error {
	parsed, err := url.Parse(subscribeURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be https, got %q", parsed.Scheme)
	}
	if !snsSubscribeURLHostRe.MatchString(parsed.Host) {
		return fmt.Errorf("host %q is not a valid SNS endpoint", parsed.Host)
	}
	return nil
}

func badRequestError(action string, err error) error {
	return &TranslateError{
		code: errorCodeBadRequest,
		err:  fmt.Errorf("%s: %w", action, err),
	}
}
