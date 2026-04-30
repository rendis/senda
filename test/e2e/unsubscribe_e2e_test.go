//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// extractTokenFromListUnsub extracts the raw token from a List-Unsubscribe header value like
// "<https://host/api/v1/u/TOKEN>" or "<https://host/api/v1/u/TOKEN/all>".
func extractTokenFromListUnsub(t *testing.T, value string) string {
	t.Helper()
	raw := strings.TrimSpace(value)
	raw = strings.TrimPrefix(raw, "<")
	raw = strings.TrimSuffix(raw, ">")
	// Handle comma-separated multi-value headers: take the first URL-shaped entry.
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "<")
		part = strings.TrimSuffix(part, ">")
		if idx := strings.Index(part, "/api/v1/u/"); idx >= 0 {
			token := part[idx+len("/api/v1/u/"):]
			// Strip any trailing path segments (e.g. "/all" appended for opt-out-all).
			token = strings.SplitN(token, "/", 2)[0]
			token = strings.TrimSpace(token)
			if token != "" {
				return token
			}
		}
	}
	t.Fatalf("extractTokenFromListUnsub: no /api/v1/u/ token found in: %q", value)
	return ""
}

// listUnsubHeaders reads List-Unsubscribe and List-Unsubscribe-Post values from a Message.
// Mailpit exposes these as a structured ListUnsubscribe field on the message object,
// not in a generic Headers map.
func listUnsubHeaders(msg *Message) (listUnsub, listUnsubPost string) {
	return strings.TrimSpace(msg.ListUnsubscribe.Header),
		strings.TrimSpace(msg.ListUnsubscribe.HeaderPost)
}

// findMessageFor returns the message ID for the first message addressed to recipient, or "".
func findMessageFor(messages []MessageSummary, recipient string) string {
	for _, m := range messages {
		for _, addr := range m.To {
			if strings.EqualFold(addr.Address, recipient) {
				return m.ID
			}
		}
	}
	return ""
}

// TestUnsubscribe_BulkTemplate_HeadersAndOneClickPipeline verifies that:
//   - A bulk template produces List-Unsubscribe / List-Unsubscribe-Post headers.
//   - The one-click POST endpoint returns 200 and is idempotent.
//   - A follow-up send to the same recipient is suppressed.
//   - A follow-up send to a different recipient is still delivered.
func TestUnsubscribe_BulkTemplate_HeadersAndOneClickPipeline(t *testing.T) {
	EnsureSetup(t)
	cli := NewTestClient(t)
	cli.LoginAs(SuperadminEmail)
	mp := NewMailpitClient(t)
	mp.ClearMessages()

	typeSlug := fmt.Sprintf("unsub-bulk-%d", time.Now().UnixNano())
	cli.CreateTemplateType(t, TemplateTypeInput{
		Slug:   typeSlug,
		Name:   "Unsubscribe Bulk Test",
		IsBulk: true,
	})

	mjml := `<mjml><mj-body><mj-section><mj-column>
		<mj-text>Hi {{ event.name }}</mj-text>
		<mj-text><a href="{{ system.unsubscribe_url }}">unsub</a></mj-text>
	</mj-column></mj-section></mj-body></mjml>`
	cli.CreateAndPublishTemplate(t, typeSlug, "unsub-bulk", mjml)

	recipient := fmt.Sprintf("recipient+%d@e2e.test", time.Now().UnixNano())
	cli.SendEmail(t, SendEmailInput{
		TemplateTypeSlug: typeSlug,
		TemplateSlug:     "unsub-bulk",
		Recipient:        recipient,
		Variables:        map[string]any{"name": "E2E"},
	})

	// Poll Mailpit for the message.
	var msgID string
	require.Eventually(t, func() bool {
		msgID = findMessageFor(mp.GetMessages(), recipient)
		return msgID != ""
	}, 45*time.Second, 500*time.Millisecond, "email never arrived for %s", recipient)

	// Read full message and verify List-Unsubscribe headers.
	msg := mp.GetMessage(msgID)
	listUnsub, listUnsubPost := listUnsubHeaders(msg)

	require.NotEmpty(t, listUnsub, "List-Unsubscribe header must be present on bulk template email")
	require.Contains(t, listUnsubPost, "List-Unsubscribe=One-Click",
		"List-Unsubscribe-Post must contain RFC 8058 value")
	require.True(t,
		strings.HasPrefix(listUnsub, "<http://") || strings.HasPrefix(listUnsub, "<https://"),
		"List-Unsubscribe must be a wrapped URL, got: %s", listUnsub)

	token := extractTokenFromListUnsub(t, listUnsub)
	require.NotEmpty(t, token, "token must be extractable from List-Unsubscribe: %s", listUnsub)

	// POST one-click — must return 200 and be idempotent.
	resp1 := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token, nil)
	require.Equal(t, http.StatusOK, resp1.StatusCode,
		"one-click first POST must be 200; body: %s", ReadResponseBody(t, resp1))
	resp1.Body.Close()

	resp2 := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token, nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "one-click POST is idempotent")
	resp2.Body.Close()

	// Send another email of the SAME type to the SAME recipient — must be suppressed.
	mp.ClearMessages()
	cli.SendEmail(t, SendEmailInput{
		TemplateTypeSlug: typeSlug,
		TemplateSlug:     "unsub-bulk",
		Recipient:        recipient,
		Variables:        map[string]any{"name": "Again"},
	})
	time.Sleep(3 * time.Second)
	require.Empty(t, findMessageFor(mp.GetMessages(), recipient),
		"second send to %s must be suppressed after unsubscribe", recipient)

	// Send to a DIFFERENT recipient of the same type — must still be delivered.
	other := fmt.Sprintf("other+%d@e2e.test", time.Now().UnixNano())
	cli.SendEmail(t, SendEmailInput{
		TemplateTypeSlug: typeSlug,
		TemplateSlug:     "unsub-bulk",
		Recipient:        other,
		Variables:        map[string]any{"name": "Other"},
	})
	require.Eventually(t, func() bool {
		return findMessageFor(mp.GetMessages(), other) != ""
	}, 45*time.Second, 500*time.Millisecond, "uninvolved recipient %s must still receive email", other)
}

// TestUnsubscribe_TransactionalTemplate_NoHeaders verifies that a non-bulk template
// does NOT emit List-Unsubscribe or List-Unsubscribe-Post headers.
func TestUnsubscribe_TransactionalTemplate_NoHeaders(t *testing.T) {
	EnsureSetup(t)
	cli := NewTestClient(t)
	cli.LoginAs(SuperadminEmail)
	mp := NewMailpitClient(t)
	mp.ClearMessages()

	typeSlug := fmt.Sprintf("unsub-tx-%d", time.Now().UnixNano())
	cli.CreateTemplateType(t, TemplateTypeInput{
		Slug:   typeSlug,
		Name:   "Transactional",
		IsBulk: false,
	})

	mjml := `<mjml><mj-body><mj-section><mj-column><mj-text>Hi there</mj-text></mj-column></mj-section></mj-body></mjml>`
	cli.CreateAndPublishTemplate(t, typeSlug, "tx", mjml)

	recipient := fmt.Sprintf("tx+%d@e2e.test", time.Now().UnixNano())
	cli.SendEmail(t, SendEmailInput{
		TemplateTypeSlug: typeSlug,
		TemplateSlug:     "tx",
		Recipient:        recipient,
	})

	var msgID string
	require.Eventually(t, func() bool {
		msgID = findMessageFor(mp.GetMessages(), recipient)
		return msgID != ""
	}, 45*time.Second, 500*time.Millisecond, "email never arrived for %s", recipient)

	msg := mp.GetMessage(msgID)
	listUnsub, listUnsubPost := listUnsubHeaders(msg)
	require.Empty(t, listUnsub,
		"transactional templates must NOT carry List-Unsubscribe header")
	require.Empty(t, listUnsubPost,
		"transactional templates must NOT carry List-Unsubscribe-Post header")
}

// TestUnsubscribe_OptOutAll_BlocksAllTypes verifies that a POST to /api/v1/u/:token/all
// suppresses subsequent emails of ANY type to that recipient.
func TestUnsubscribe_OptOutAll_BlocksAllTypes(t *testing.T) {
	EnsureSetup(t)
	cli := NewTestClient(t)
	cli.LoginAs(SuperadminEmail)
	mp := NewMailpitClient(t)
	mp.ClearMessages()

	type1 := fmt.Sprintf("all-1-%d", time.Now().UnixNano())
	type2 := fmt.Sprintf("all-2-%d", time.Now().UnixNano())

	for _, slug := range []string{type1, type2} {
		cli.CreateTemplateType(t, TemplateTypeInput{Slug: slug, Name: slug, IsBulk: true})
		mjml := fmt.Sprintf(
			`<mjml><mj-body><mj-section><mj-column><mj-text>%s {{ system.unsubscribe_url }}</mj-text></mj-column></mj-section></mj-body></mjml>`,
			slug,
		)
		cli.CreateAndPublishTemplate(t, slug, slug, mjml)
	}

	recipient := fmt.Sprintf("all+%d@e2e.test", time.Now().UnixNano())

	// Send first bulk type to get a token.
	cli.SendEmail(t, SendEmailInput{TemplateTypeSlug: type1, TemplateSlug: type1, Recipient: recipient})

	var msgID string
	require.Eventually(t, func() bool {
		msgID = findMessageFor(mp.GetMessages(), recipient)
		return msgID != ""
	}, 45*time.Second, 500*time.Millisecond, "first email never arrived for %s", recipient)

	msg := mp.GetMessage(msgID)
	listUnsub, _ := listUnsubHeaders(msg)
	require.NotEmpty(t, listUnsub, "List-Unsubscribe header missing on bulk type1 email")

	token := extractTokenFromListUnsub(t, listUnsub)
	require.NotEmpty(t, token)

	// Opt out of ALL.
	resp := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token+"/all", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"opt-out-all must return 200; body: %s", ReadResponseBody(t, resp))
	resp.Body.Close()

	// Send second (different) bulk type to same recipient — must be blocked.
	mp.ClearMessages()
	cli.SendEmail(t, SendEmailInput{TemplateTypeSlug: type2, TemplateSlug: type2, Recipient: recipient})
	time.Sleep(3 * time.Second)
	require.Empty(t, findMessageFor(mp.GetMessages(), recipient),
		"after opt-out-all, NO type may reach the recipient")
}
