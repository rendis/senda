package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

func TestSuppressionBatchEvaluator_EvaluateSeparatesAcceptedAndSuppressedRecipients(t *testing.T) {
	store := &mockSuppressionStoreSend{
		checkBatchFn: func(_ context.Context, _ uuid.UUID, emails []string) (map[string]string, error) {
			if len(emails) != 5 {
				t.Fatalf("expected 5 unique emails in batch, got %d: %v", len(emails), emails)
			}
			return map[string]string{
				"suppressed-to@user.com":  "hard_bounce",
				"suppressed-cc@user.com":  "complaint",
				"suppressed-bcc@user.com": "manual",
			}, nil
		},
	}

	evaluator := service.NewSuppressionBatchEvaluator(store)

	result, err := evaluator.Evaluate(
		context.Background(),
		uuid.New(),
		[]string{"accepted@user.com", "suppressed-to@user.com"},
		[]string{"clean-cc@user.com", "suppressed-cc@user.com"},
		[]string{"suppressed-bcc@user.com"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.To) != 2 {
		t.Fatalf("expected 2 to decisions, got %d", len(result.To))
	}
	if result.To[0].Address != "accepted@user.com" || result.To[0].Suppressed {
		t.Fatalf("expected accepted@user.com to stay accepted, got %#v", result.To[0])
	}
	if result.To[1].Address != "suppressed-to@user.com" || !result.To[1].Suppressed || result.To[1].Reason != "hard_bounce" {
		t.Fatalf("expected suppressed-to@user.com to be suppressed with hard_bounce, got %#v", result.To[1])
	}
	if len(result.CC) != 1 || result.CC[0] != "clean-cc@user.com" {
		t.Fatalf("expected only clean CC to remain, got %v", result.CC)
	}
	if len(result.BCC) != 0 {
		t.Fatalf("expected all BCC addresses filtered out, got %v", result.BCC)
	}
}

func TestSuppressionBatchEvaluator_EvaluatePropagatesBatchError(t *testing.T) {
	store := &mockSuppressionStoreSend{
		checkBatchFn: func(_ context.Context, _ uuid.UUID, _ []string) (map[string]string, error) {
			return nil, errors.New("store unavailable")
		},
	}

	evaluator := service.NewSuppressionBatchEvaluator(store)

	_, err := evaluator.Evaluate(context.Background(), uuid.New(), []string{"alice@user.com"}, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "evaluate suppression batch: store unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSuppressionBatchEvaluator_EvaluateMatchesRecipientsCaseInsensitively(t *testing.T) {
	store := &mockSuppressionStoreSend{
		checkBatchFn: func(_ context.Context, _ uuid.UUID, emails []string) (map[string]string, error) {
			want := []string{"mixed@user.com", "clean-cc@user.com", "shadow@user.com"}
			if len(emails) != len(want) {
				t.Fatalf("expected %d normalized emails, got %d: %v", len(want), len(emails), emails)
			}
			for i, email := range want {
				if emails[i] != email {
					t.Fatalf("emails[%d] = %q, want %q", i, emails[i], email)
				}
			}
			return map[string]string{
				"mixed@user.com":  "hard_bounce",
				"shadow@user.com": "complaint",
			}, nil
		},
	}

	evaluator := service.NewSuppressionBatchEvaluator(store)

	result, err := evaluator.Evaluate(
		context.Background(),
		uuid.New(),
		[]string{"Mixed@User.com"},
		[]string{"clean-cc@user.com"},
		[]string{"SHADOW@USER.COM"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.To) != 1 || !result.To[0].Suppressed {
		t.Fatalf("expected Mixed@User.com to be suppressed, got %+v", result.To)
	}
	if result.To[0].Address != "Mixed@User.com" {
		t.Fatalf("expected original casing to be preserved, got %q", result.To[0].Address)
	}
	if result.To[0].Reason != "hard_bounce" {
		t.Fatalf("expected hard_bounce reason, got %q", result.To[0].Reason)
	}
	if len(result.CC) != 1 || result.CC[0] != "clean-cc@user.com" {
		t.Fatalf("expected clean CC to remain, got %v", result.CC)
	}
	if len(result.BCC) != 0 {
		t.Fatalf("expected SHADOW@USER.COM to be filtered from BCC, got %v", result.BCC)
	}
}

func TestSendService_Send_UsesSuppressionBatchInsteadOfSequentialChecks(t *testing.T) {
	f := newSendFixture()
	batchCalls := 0
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		return false, "", errors.New("unexpected sequential suppression check for " + email)
	}
	f.suppression.checkBatchFn = func(_ context.Context, _ uuid.UUID, emails []string) (map[string]string, error) {
		batchCalls++
		if len(emails) != 5 {
			t.Fatalf("expected 5 unique emails in batch, got %d: %v", len(emails), emails)
		}
		return map[string]string{
			"bob@user.com":           "hard_bounce",
			"suppressed-cc@user.com": "complaint",
		}, nil
	}

	svc := f.buildService()
	req := f.happyRequest()
	req.To = []string{"alice@user.com", "bob@user.com"}
	req.CC = []string{"clean-cc@user.com", "suppressed-cc@user.com"}
	req.BCC = []string{"clean-bcc@user.com"}

	resp, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if batchCalls != 1 {
		t.Fatalf("expected a single batch suppression lookup, got %d", batchCalls)
	}
	if len(resp.TrackingIDs) != 2 {
		t.Fatalf("expected 2 tracking results, got %d", len(resp.TrackingIDs))
	}
	if resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected alice accepted, got %q", resp.TrackingIDs[0].Status)
	}
	if resp.TrackingIDs[1].Status != "suppressed" {
		t.Fatalf("expected bob suppressed, got %q", resp.TrackingIDs[1].Status)
	}
	if len(f.emailStore.emails) != 2 {
		t.Fatalf("expected 2 email records, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].CC; len(got) != 1 || got[0] != "clean-cc@user.com" {
		t.Fatalf("expected filtered CC on accepted email, got %v", got)
	}
	if got := f.emailStore.emails[0].BCC; len(got) != 1 || got[0] != "clean-bcc@user.com" {
		t.Fatalf("expected clean BCC on accepted email, got %v", got)
	}
	if got := f.emailStore.emails[1].Status; got != "suppressed" {
		t.Fatalf("expected suppressed email status, got %q", got)
	}
}

func TestSendService_SendBatch_CanonicalizesSuppressionStatusBatchLookups(t *testing.T) {
	f := newSendFixture()
	f.suppression.isSuppressedFn = func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
		t.Fatalf("unexpected sequential suppression check for %s", email)
		return false, "", nil
	}
	f.suppression.getStatusesFn = func(_ context.Context, _ uuid.UUID, emails []string) (map[string]port.SuppressionStatus, error) {
		want := []string{"mixed@user.com", "clean-cc@user.com", "shadow@user.com"}
		if len(emails) != len(want) {
			t.Fatalf("expected %d canonical emails, got %d: %v", len(want), len(emails), emails)
		}
		for i, email := range want {
			if emails[i] != email {
				t.Fatalf("emails[%d] = %q, want %q", i, emails[i], email)
			}
		}

		return map[string]port.SuppressionStatus{
			"mixed@user.com":  {Suppressed: true, Reason: "hard_bounce"},
			"shadow@user.com": {Suppressed: true, Reason: "complaint"},
		}, nil
	}

	svc := f.buildService()
	resp, err := svc.SendBatch(context.Background(), &service.SendBatchRequest{
		Ref: "latam:acme:welcome",
		Items: []service.SendBatchItemRequest{{
			To:        "Mixed@User.com",
			CC:        []string{"clean-cc@user.com"},
			BCC:       []string{"SHADOW@USER.COM"},
			Variables: map[string]any{"name": "Mixed"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resp.Status; got != "accepted" {
		t.Fatalf("expected accepted batch response status, got %q", got)
	}
	if resp.AcceptedCount != 0 || resp.SuppressedCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("unexpected batch counters: %+v", resp)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item result, got %d", len(resp.Items))
	}
	if got := resp.Items[0].Status; got != "suppressed" {
		t.Fatalf("expected suppressed response status, got %q", got)
	}
	if len(f.emailStore.emails) != 1 {
		t.Fatalf("expected 1 persisted email, got %d", len(f.emailStore.emails))
	}
	if got := f.emailStore.emails[0].Status; got != "suppressed" {
		t.Fatalf("expected persisted suppressed status, got %q", got)
	}
	if got := f.emailStore.emails[0].BCC; len(got) != 0 {
		t.Fatalf("expected suppressed BCC recipient to be filtered out, got %v", got)
	}
}

func TestSuppressionBatchEvaluator_EvaluateMany_CanonicalizesBatchOnlyOnce(t *testing.T) {
	store := &mockSuppressionStoreSend{
		checkBatchFn: func(_ context.Context, _ uuid.UUID, emails []string) (map[string]string, error) {
			want := []string{
				"alice@user.com",
				"shared@user.com",
				"hidden@user.com",
				"bob@user.com",
			}
			if len(emails) != len(want) {
				t.Fatalf("expected %d canonical emails, got %d: %v", len(want), len(emails), emails)
			}
			for i, email := range want {
				if emails[i] != email {
					t.Fatalf("emails[%d] = %q, want %q", i, emails[i], email)
				}
			}
			return map[string]string{}, nil
		},
	}

	evaluator := service.NewSuppressionBatchEvaluator(store)

	_, err := evaluator.EvaluateMany(context.Background(), uuid.New(), []service.SuppressionBatchInput{
		{
			To:  []string{"Alice@User.com"},
			CC:  []string{"shared@user.com"},
			BCC: []string{"HIDDEN@USER.COM"},
		},
		{
			To:  []string{"alice@user.com", "BOB@USER.COM"},
			CC:  []string{"Shared@User.com"},
			BCC: []string{"hidden@user.com"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
