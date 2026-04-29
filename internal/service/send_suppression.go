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

type templateTypeOptOutStore interface {
	BatchCheckOptOut(ctx context.Context, workspaceID, templateTypeID uuid.UUID, emails []string) (map[string]struct{}, error)
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
	tts   templateTypeOptOutStore
}

func NewSuppressionBatchEvaluator(store suppressionBatchStore) *SuppressionBatchEvaluator {
	return &SuppressionBatchEvaluator{store: store}
}

// WithTemplateTypeStore enables a third suppression layer: per (workspace, template_type, email)
// opt-outs. Returns the evaluator for chaining.
func (e *SuppressionBatchEvaluator) WithTemplateTypeStore(ts templateTypeOptOutStore) *SuppressionBatchEvaluator {
	e.tts = ts
	return e
}

// EvaluateForType performs the standard workspace-level evaluation, then layers
// per-(template_type, email) opt-outs on top. Workspace suppressions take
// precedence — their reason is preserved. Recipients only opted-out at the
// template-type level get Reason="type_optout".
//
// If no template-type store has been attached via WithTemplateTypeStore, this
// method degrades to the legacy Evaluate behaviour.
func (e *SuppressionBatchEvaluator) EvaluateForType(
	ctx context.Context,
	workspaceID, templateTypeID uuid.UUID,
	to, cc, bcc []string,
) (*SuppressionBatchEvaluation, error) {
	base, err := e.Evaluate(ctx, workspaceID, to, cc, bcc)
	if err != nil {
		return nil, err
	}
	if e.tts == nil {
		return base, nil
	}

	// Collect addresses not already suppressed at workspace level — those are the
	// only ones we need to check against the template-type opt-out store.
	canonical := make([]string, 0, len(base.To)+len(base.CC)+len(base.BCC))
	seen := make(map[string]struct{})
	for _, d := range base.To {
		if d.Suppressed {
			continue
		}
		c := domain.CanonicalRecipientAddress(d.Address)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		canonical = append(canonical, c)
	}
	for _, addr := range base.CC {
		c := domain.CanonicalRecipientAddress(addr)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		canonical = append(canonical, c)
	}
	for _, addr := range base.BCC {
		c := domain.CanonicalRecipientAddress(addr)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		canonical = append(canonical, c)
	}

	optOuts, err := e.tts.BatchCheckOptOut(ctx, workspaceID, templateTypeID, canonical)
	if err != nil {
		return nil, fmt.Errorf("evaluate type opt-outs: %w", err)
	}
	if len(optOuts) == 0 {
		return base, nil
	}

	for i, d := range base.To {
		if d.Suppressed {
			continue
		}
		if _, ok := optOuts[domain.CanonicalRecipientAddress(d.Address)]; ok {
			base.To[i].Suppressed = true
			base.To[i].Reason = "type_optout"
		}
	}
	base.CC = filterTypeOptOuts(base.CC, optOuts)
	base.BCC = filterTypeOptOuts(base.BCC, optOuts)
	return base, nil
}

func filterTypeOptOuts(addrs []string, optOuts map[string]struct{}) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if _, ok := optOuts[domain.CanonicalRecipientAddress(a)]; ok {
			continue
		}
		out = append(out, a)
	}
	return out
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
