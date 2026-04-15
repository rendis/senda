package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/adapter/sns"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

const snsHTTPTimeout = 10 * time.Second

func newSNSHTTPClient() *http.Client {
	return &http.Client{Timeout: snsHTTPTimeout}
}

func buildSESWebhookHandler(cfg *config.Config, processor *service.EventProcessor, logger *slog.Logger, replayStore ...port.SNSReplayStore) (*handler.SESWebhookHandler, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	snsVerifier := sns.NewVerifier(newSNSHTTPClient())
	snsConfirmer := handler.NewHTTPSubscriptionConfirmer(newSNSHTTPClient())

	var store port.SNSReplayStore
	if len(replayStore) > 0 {
		store = replayStore[0]
	}

	opts := []handler.SESWebhookHandlerOption{
		handler.WithSkipSignatureVerification(cfg.SNS.SkipSignatureVerification),
		handler.WithSNSReplayStore(store, cfg.SNS.ReplayWindow),
	}
	if cfg.SNS.ExpectedTopicArn != "" {
		opts = append(opts, handler.WithExpectedSNSDestination(cfg.SNS.ExpectedTopicArn, cfg.SNS.ExpectedAccountID))
	}

	return handler.NewSESWebhookHandler(
		processor,
		snsVerifier,
		snsConfirmer,
		logger,
		opts...,
	), nil
}

func buildMediaHandler(cfg *config.Config, logger *slog.Logger) *handler.MediaHandler {
	allowedHosts := []string{"img.youtube.com", "i.ytimg.com"}
	if cfg != nil && len(cfg.Media.ThumbnailAllowedHosts) > 0 {
		allowedHosts = cfg.Media.ThumbnailAllowedHosts
	}

	ttl := 24 * time.Hour
	maxEntries := 500
	timeout := 10 * time.Second
	if cfg != nil {
		if cfg.Media.ThumbnailCacheTTL > 0 {
			ttl = cfg.Media.ThumbnailCacheTTL
		}
		if cfg.Media.ThumbnailCacheMaxEntries > 0 {
			maxEntries = cfg.Media.ThumbnailCacheMaxEntries
		}
		if cfg.Media.ThumbnailFetchTimeout > 0 {
			timeout = cfg.Media.ThumbnailFetchTimeout
		}
	}

	return handler.NewMediaHandler(
		logger,
		handler.WithAllowedThumbnailHosts(allowedHosts...),
		handler.WithThumbnailCachePolicy(ttl, maxEntries),
		handler.WithThumbnailFetchTimeout(timeout),
	)
}
