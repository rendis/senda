package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
)

type suppressionBatchStore interface {
	CheckBatch(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]string, error)
}

type SuppressionRecipientDecision struct {
	Address    string
	Suppressed bool
	Reason     string
}

type SuppressionBatchEvaluation struct {
	To  []SuppressionRecipientDecision
	CC  []string
	BCC []string
}

type SuppressionBatchInput struct {
	To  []string
	CC  []string
	BCC []string
}

type SuppressionBatchEvaluator struct {
	store suppressionBatchStore
}

func NewSuppressionBatchEvaluator(store suppressionBatchStore) *SuppressionBatchEvaluator {
	return &SuppressionBatchEvaluator{store: store}
}

func (e *SuppressionBatchEvaluator) Evaluate(
	ctx context.Context,
	wsID uuid.UUID,
	to []string,
	cc []string,
	bcc []string,
) (*SuppressionBatchEvaluation, error) {
	results, err := e.EvaluateMany(ctx, wsID, []SuppressionBatchInput{{
		To:  to,
		CC:  cc,
		BCC: bcc,
	}})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

func (e *SuppressionBatchEvaluator) EvaluateMany(
	ctx context.Context,
	wsID uuid.UUID,
	inputs []SuppressionBatchInput,
) ([]*SuppressionBatchEvaluation, error) {
	lookup := make([]string, 0)
	seen := make(map[string]struct{})
	for _, input := range inputs {
		lookup = appendUniqueCanonicalRecipients(lookup, seen, input.To, input.CC, input.BCC)
	}

	suppressedByEmail, err := e.store.CheckBatch(ctx, wsID, lookup)
	if err != nil {
		return nil, fmt.Errorf("evaluate suppression batch: %w", err)
	}

	results := make([]*SuppressionBatchEvaluation, 0, len(inputs))
	for _, input := range inputs {
		results = append(results, projectSuppressionBatch(input, suppressedByEmail))
	}

	return results, nil
}

func appendUniqueCanonicalRecipients(dst []string, seen map[string]struct{}, groups ...[]string) []string {
	for _, group := range groups {
		for _, recipient := range group {
			if canonical := domain.CanonicalRecipientAddress(recipient); canonical != "" {
				if _, ok := seen[canonical]; ok {
					continue
				}
				seen[canonical] = struct{}{}
				dst = append(dst, canonical)
			}
		}
	}
	return dst
}

func projectSuppressionBatch(input SuppressionBatchInput, suppressedByEmail map[string]string) *SuppressionBatchEvaluation {
	result := &SuppressionBatchEvaluation{
		To:  make([]SuppressionRecipientDecision, 0, len(input.To)),
		CC:  make([]string, 0, len(input.CC)),
		BCC: make([]string, 0, len(input.BCC)),
	}

	for _, recipient := range input.To {
		reason, suppressed := suppressedByEmail[domain.CanonicalRecipientAddress(recipient)]
		result.To = append(result.To, SuppressionRecipientDecision{
			Address:    recipient,
			Suppressed: suppressed,
			Reason:     reason,
		})
	}

	for _, recipient := range input.CC {
		if _, suppressed := suppressedByEmail[domain.CanonicalRecipientAddress(recipient)]; !suppressed {
			result.CC = append(result.CC, recipient)
		}
	}

	for _, recipient := range input.BCC {
		if _, suppressed := suppressedByEmail[domain.CanonicalRecipientAddress(recipient)]; !suppressed {
			result.BCC = append(result.BCC, recipient)
		}
	}

	return result
}
