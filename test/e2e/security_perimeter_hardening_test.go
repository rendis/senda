//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/stretchr/testify/require"
)

const (
	hardeningSNSExpectedTopicArn  = "arn:aws:sns:us-east-1:123456789012:SES-Events"
	hardeningSNSExpectedAccountID = "123456789012"
	hardeningSNSReplayWindow      = 15 * time.Minute
)

type hardeningReplayStore struct {
	mu     sync.Mutex
	claims map[string]time.Time
}

func (s *hardeningReplayStore) Claim(_ context.Context, topicArn, messageID string, messageTimestamp time.Time, replayWindow time.Duration) (port.SNSReplayDecision, error) {
	if s == nil {
		return port.SNSReplayDecisionAccepted, nil
	}
	if s.claims == nil {
		s.claims = make(map[string]time.Time)
	}
	if time.Since(messageTimestamp) > replayWindow {
		return port.SNSReplayDecisionStale, nil
	}

	key := topicArn + "|" + messageID
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.claims[key]; ok {
		return port.SNSReplayDecisionDuplicate, nil
	}
	s.claims[key] = messageTimestamp
	return port.SNSReplayDecisionAccepted, nil
}

type hardeningEventProcessor struct {
	mu                   sync.Mutex
	email                *domain.Email
	lookupCount          int
	updateCount          int
	eventCount           int
	suppressionGlobal    int
	suppressionWorkspace int
}

func (h *hardeningEventProcessor) GetByProviderMessageID(_ context.Context, providerMessageID string) (*domain.Email, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.email == nil || h.email.ProviderMessageID == nil || *h.email.ProviderMessageID != providerMessageID {
		return nil, errors.New("email not found")
	}
	h.lookupCount++
	return h.email, nil
}

func (h *hardeningEventProcessor) UpdateStatus(_ context.Context, _ uuid.UUID, newStatus, _ domain.EmailStatus) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.updateCount++
	if h.email != nil {
		h.email.Status = newStatus
	}
	return nil
}

func (h *hardeningEventProcessor) AddEvent(_ context.Context, _ *domain.EmailEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.eventCount++
	return nil
}

func (h *hardeningEventProcessor) AddGlobal(_ context.Context, _ *domain.SuppressionGlobal) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.suppressionGlobal++
	return nil
}

func (h *hardeningEventProcessor) AddWorkspace(_ context.Context, _ *domain.SuppressionWorkspace) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.suppressionWorkspace++
	return nil
}

func (h *hardeningEventProcessor) snapshot() (lookups, updates, events int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lookupCount, h.updateCount, h.eventCount
}

type hardeningExternalStore struct {
	profile  domain.ExternalIntegrationProfile
	auth     port.ExternalAuthMethod
	resolver port.ExternalWorkspaceResolver
}

func (s *hardeningExternalStore) LoadProfileBySlug(_ context.Context, slug string) (domain.ExternalIntegrationProfile, error) {
	if strings.TrimSpace(strings.ToLower(slug)) != s.profile.Slug {
		return domain.ExternalIntegrationProfile{}, domain.ErrNotFound
	}
	return s.profile, nil
}

func (s *hardeningExternalStore) AuthMethodByName(name string) (port.ExternalAuthMethod, bool) {
	if s.auth != nil && strings.EqualFold(name, s.auth.Name()) {
		return s.auth, true
	}
	return nil, false
}

func (s *hardeningExternalStore) ResolverByName(name string) (port.ExternalWorkspaceResolver, bool) {
	if s.resolver != nil && strings.EqualFold(name, s.resolver.Name()) {
		return s.resolver, true
	}
	return nil, false
}

type hardeningExternalAuth struct{}

func (hardeningExternalAuth) Name() string        { return "signed-headers" }
func (hardeningExternalAuth) Description() string { return "test auth" }
func (hardeningExternalAuth) Authenticate(context.Context, *port.ExternalIntegrationRequest) (*port.ExternalAuthResult, error) {
	return &port.ExternalAuthResult{}, nil
}

type hardeningExternalResolver struct{}

func (hardeningExternalResolver) Name() string        { return "tenant-workspace-resolver" }
func (hardeningExternalResolver) Description() string { return "test resolver" }
func (hardeningExternalResolver) ResolveWorkspace(context.Context, *port.ExternalIntegrationRequest, *port.ExternalAuthResult, port.WorkspaceFilter) (*port.ExternalWorkspaceResolution, error) {
	return &port.ExternalWorkspaceResolution{WorkspaceCode: "marketing", ReadOnly: false}, nil
}

func TestSecurityPerimeterHardening01_AutonomousFlow(t *testing.T) {
	t.Run("sns_topic_account_replay_boundaries", func(t *testing.T) {
		eventHarness := &hardeningEventProcessor{
			email: &domain.Email{
				ID:                uuid.Must(uuid.NewV7()),
				WorkspaceID:       uuid.Must(uuid.NewV7()),
				TenantID:          uuid.Must(uuid.NewV7()),
				Status:            domain.StatusSent,
				RecipientEmail:    "recipient@example.com",
				TrackingID:        "trk-hardening-1",
				ProviderMessageID: ptr("provider-message-1"),
			},
		}
		snsHandler := handler.NewSESWebhookHandler(
			service.NewEventProcessor(eventHarness, eventHarness, eventHarness, nil, slog.New(slog.NewTextHandler(io.Discard, nil))),
			nil,
			nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			handler.WithSkipSignatureVerification(true),
			handler.WithExpectedSNSDestination(hardeningSNSExpectedTopicArn, hardeningSNSExpectedAccountID),
			handler.WithSNSReplayStore(&hardeningReplayStore{}, hardeningSNSReplayWindow),
		)

		e := echo.New()
		e.HTTPErrorHandler = response.HTTPErrorHandler
		e.POST("/api/v1/webhooks/ses/inbound", snsHandler.HandleInbound)

		t.Run("rejects topic_or_account_mismatch", func(t *testing.T) {
			body := snsRequestBody(t, hardeningSNSExpectedTopicArn, "provider-message-2", time.Now().UTC(), buildSESNotification(t, "Delivery", "provider-message-2", "recipient@example.com", time.Now().UTC()))
			body = bytes.Replace(body, []byte(hardeningSNSExpectedTopicArn), []byte("arn:aws:sns:us-east-1:999999999999:SES-Events"), 1)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			lookups, updates, events := eventHarness.snapshot()
			require.Zero(t, lookups)
			require.Zero(t, updates)
			require.Zero(t, events)
		})

		t.Run("accepts_first_notification_then_blocks_duplicate_and_stale_replay", func(t *testing.T) {
			now := time.Now().UTC()
			first := snsRequestBody(t, hardeningSNSExpectedTopicArn, "msg-duplicate", now, buildSESNotification(t, "Delivery", "provider-message-1", "recipient@example.com", now))
			duplicate := append([]byte(nil), first...)
			stale := snsRequestBody(t, hardeningSNSExpectedTopicArn, "msg-stale", now.Add(-time.Hour), buildSESNotification(t, "Delivery", "provider-message-1", "recipient@example.com", now.Add(-time.Hour)))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(first))
			req.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(duplicate))
			req.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(stale))
			req.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			lookups, updates, events := eventHarness.snapshot()
			require.Equal(t, 1, lookups)
			require.Equal(t, 1, updates)
			require.Equal(t, 1, events)
		})
	})

	t.Run("external_integration_rejects_query_token_transport", func(t *testing.T) {
		store := &hardeningExternalStore{
			profile: domain.ExternalIntegrationProfile{
				Slug:            "partner-portal",
				Name:            "Partner Portal",
				Enabled:         true,
				AuthMethodName:  "signed-headers",
				ResolverName:    "tenant-workspace-resolver",
				AllowedHeaders:  []string{"X-Tenant-Code"},
				RequiredHeaders: []string{"X-Tenant-Code"},
			},
			auth:     hardeningExternalAuth{},
			resolver: hardeningExternalResolver{},
		}

		e := echo.New()
		e.HTTPErrorHandler = response.HTTPErrorHandler
		e.Use(middleware.ExternalIntegration(store))
		e.GET("/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/template-types", func(c *echo.Context) error {
			return c.NoContent(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/external/partner-portal/tenants/acme/workspaces/marketing/template-types?token=leaked-bearer-token", nil)
		req.Header.Set("X-Senda-Environment", "prod")
		req.Header.Set("X-Tenant-Code", "acme")

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("media_pins_destination_and_blocks_private_redirects", func(t *testing.T) {
		lookupCounts := map[string]int{}
		lookup := func(_ context.Context, host string) ([]net.IP, error) {
			host = strings.ToLower(strings.TrimSpace(host))
			lookupCounts[host]++
			switch host {
			case "media.example.test":
				if lookupCounts[host] == 1 {
					return []net.IP{net.ParseIP("93.184.216.34")}, nil
				}
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			default:
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			}
		}

		blackhole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "should not be reached", http.StatusInternalServerError)
		}))
		t.Cleanup(blackhole.Close)

		mux := http.NewServeMux()
		mux.HandleFunc("/redirect-private.jpg", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1/private.jpg", http.StatusFound)
		})
		mux.HandleFunc("/redirect-rebind.jpg", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://media.example.test/final.jpg", http.StatusFound)
		})
		mux.HandleFunc("/final.jpg", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_ = png.Encode(w, makeHardeningPNG(t))
		})
		upstream := httptest.NewServer(mux)
		t.Cleanup(upstream.Close)

		dial := func(_ context.Context, _ string, port string, pinnedIP net.IP) (string, error) {
			if pinnedIP == nil {
				return "", fmt.Errorf("missing pinned destination")
			}
			if pinnedIP.IsPrivate() || pinnedIP.IsLoopback() {
				return blackhole.Listener.Addr().String(), nil
			}
			return upstream.Listener.Addr().String(), nil
		}

		h := handler.NewMediaHandler(
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			handler.WithAllowedHosts("media.example.test"),
			handler.WithLookupIPFunc(lookup),
			handler.WithDialAddressFunc(dial),
		)
		e := echo.New()
		e.HTTPErrorHandler = response.HTTPErrorHandler
		e.GET("/public/video-thumbnail", h.HandleVideoThumbnail)

		t.Run("private_redirect_is_denied", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://media.example.test/redirect-private.jpg"), nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusBadGateway, "unexpected status %d", rec.Code)
		})

		t.Run("rebinding_uses_pinned_destination", func(t *testing.T) {
			lookupCounts := map[string]int{}
			lookup := func(_ context.Context, host string) ([]net.IP, error) {
				host = strings.ToLower(strings.TrimSpace(host))
				lookupCounts[host]++
				if host == "media.example.test" && lookupCounts[host] == 1 {
					return []net.IP{net.ParseIP("93.184.216.34")}, nil
				}
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			}
			h := handler.NewMediaHandler(
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				handler.WithAllowedHosts("media.example.test"),
				handler.WithLookupIPFunc(lookup),
				handler.WithDialAddressFunc(dial),
			)
			e := echo.New()
			e.HTTPErrorHandler = response.HTTPErrorHandler
			e.GET("/public/video-thumbnail", h.HandleVideoThumbnail)

			req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://media.example.test/redirect-rebind.jpg"), nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
			require.Equal(t, 1, lookupCounts["media.example.test"])
		})

		t.Run("mixed_dns_answers_fail_closed", func(t *testing.T) {
			tests := []struct {
				name string
				ips  []net.IP
			}{
				{
					name: "mixed_ipv4_private",
					ips: []net.IP{
						net.ParseIP("93.184.216.34"),
						net.ParseIP("10.0.0.1"),
					},
				},
				{
					name: "mixed_ipv6_reserved",
					ips: []net.IP{
						net.ParseIP("93.184.216.34"),
						net.ParseIP("fc00::1"),
					},
				},
				{
					name: "mixed_ipv4_special_purpose",
					ips: []net.IP{
						net.ParseIP("93.184.216.34"),
						net.ParseIP("192.0.2.1"),
					},
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					lookupCalls := 0
					lookup := func(_ context.Context, host string) ([]net.IP, error) {
						lookupCalls++
						if host != "media.example.test" {
							return nil, fmt.Errorf("unexpected host %q", host)
						}
						return tc.ips, nil
					}
					dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
						return "", fmt.Errorf("dial must not be called for mixed DNS answers: host=%s port=%s ip=%v", host, port, pinnedIP)
					}
					h := handler.NewMediaHandler(
						slog.New(slog.NewTextHandler(io.Discard, nil)),
						handler.WithAllowedHosts("media.example.test"),
						handler.WithLookupIPFunc(lookup),
						handler.WithDialAddressFunc(dial),
					)
					e := echo.New()
					e.HTTPErrorHandler = response.HTTPErrorHandler
					e.GET("/public/video-thumbnail", h.HandleVideoThumbnail)

					req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://media.example.test/redirect-rebind.jpg"), nil)
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, req)

					require.Equal(t, http.StatusBadRequest, rec.Code)
					require.Equal(t, 1, lookupCalls)
				})
			}
		})

		t.Run("special_purpose_ipv4_is_blocked", func(t *testing.T) {
			tests := []struct {
				name string
				host string
				ip   string
			}{
				{name: "zero_net", host: "0.0.0.1", ip: "0.0.0.1"},
				{name: "documentation_net", host: "192.0.2.1", ip: "192.0.2.1"},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					lookup := func(_ context.Context, host string) ([]net.IP, error) {
						if host != tc.host {
							return nil, fmt.Errorf("unexpected host %q", host)
						}
						return []net.IP{net.ParseIP(tc.ip)}, nil
					}
					dial := func(_ context.Context, host, port string, pinnedIP net.IP) (string, error) {
						return "", fmt.Errorf("dial must not be called for special-purpose IPv4: host=%s port=%s ip=%v", host, port, pinnedIP)
					}
					h := handler.NewMediaHandler(
						slog.New(slog.NewTextHandler(io.Discard, nil)),
						handler.WithAllowedHosts(tc.host),
						handler.WithLookupIPFunc(lookup),
						handler.WithDialAddressFunc(dial),
					)
					e := echo.New()
					e.HTTPErrorHandler = response.HTTPErrorHandler
					e.GET("/public/video-thumbnail", h.HandleVideoThumbnail)

					req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("http://"+tc.host+"/thumb.jpg"), nil)
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, req)

					require.Equal(t, http.StatusBadRequest, rec.Code)
				})
			}
		})
	})
}

func snsRequestBody(t *testing.T, topicArn, messageID string, timestamp time.Time, message []byte) []byte {
	t.Helper()

	envelope := snsEnvelope{
		Type:      "Notification",
		MessageID: messageID,
		TopicArn:  topicArn,
		Message:   string(message),
		Timestamp: timestamp.UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	return body
}

func makeHardeningPNG(t *testing.T) image.Image {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 0x2a, G: 0x6f, B: 0xf0, A: 0xff})
		}
	}
	return img
}

func ptr[T any](v T) *T {
	return &v
}
