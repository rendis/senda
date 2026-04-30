//go:build e2e

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// screenshotEnabled reports whether the screenshot feature is configured for
// E2E testing via the SENDA_E2E_SCREENSHOT_ENABLED environment variable.
func screenshotEnabled() bool {
	return truthyEnv(os.Getenv("SENDA_E2E_SCREENSHOT_ENABLED"))
}

const builderSectionBlocksScreenshotMJML = `<mjml>
  <mj-body>
    <mj-section css-class="senda-builder-media-content" background-color="#ffffff" padding="24px 20px">
      <mj-column width="45%">
        <mj-image src="https://placehold.co/260x180/png" alt="Digital profile preview" padding="0 12px" />
      </mj-column>
      <mj-column width="55%">
        <mj-text font-size="22px" font-weight="600" color="#111827">Benefits</mj-text>
        <mj-text font-size="15px" line-height="1.5" color="#111827">Digital profile with virtual tours, testimonials, videos, and useful information for families.</mj-text>
        <mj-button href="#" background-color="#5429ff" color="#ffffff" border-radius="24px" align="left">Learn more</mj-button>
      </mj-column>
    </mj-section>
    <mj-section css-class="senda-builder-cta-group" background-color="#f4f3ff" padding="28px 20px">
      <mj-column>
        <mj-text align="center" font-size="17px" line-height="1.45" color="#111447">Invite readers to continue with one or more clear actions.</mj-text>
        <mj-button href="#" align="center" background-color="#5429ff" color="#ffffff" border-radius="24px">Primary action</mj-button>
        <mj-button href="#" align="center" background-color="#5429ff" color="#ffffff" border-radius="24px">Secondary action</mj-button>
      </mj-column>
    </mj-section>
    <mj-section css-class="senda-builder-feature-list" background-color="#f7f7ff" padding="30px 20px">
      <mj-column>
        <mj-text align="center" font-size="24px" font-weight="700" color="#5429ff">Key milestones</mj-text>
        <mj-text font-size="17px" line-height="1.45" color="#111447"><span style="display:inline-block;width:54px;font-size:34px;color:#5429ff;vertical-align:top;">*</span><span style="display:inline-block;width:calc(100% - 64px);vertical-align:top;">Digitized key educational access processes.</span></mj-text>
        <mj-text font-size="17px" line-height="1.45" color="#111447"><span style="display:inline-block;width:54px;font-size:34px;color:#5429ff;vertical-align:top;">o</span><span style="display:inline-block;width:calc(100% - 64px);vertical-align:top;">Collaborated with teams across multiple regions.</span></mj-text>
        <mj-text font-size="17px" line-height="1.45" color="#111447"><span style="display:inline-block;width:54px;font-size:34px;color:#5429ff;vertical-align:top;">+</span><span style="display:inline-block;width:calc(100% - 64px);vertical-align:top;">Strengthened strategic alliances.</span></mj-text>
      </mj-column>
    </mj-section>
    <mj-section background-color="#5429ff" padding="22px 20px">
      <mj-column>
        <mj-text align="center" color="#ffffff" font-size="18px">Thank you for your trust and collaboration.</mj-text>
      </mj-column>
    </mj-section>
    <mj-section css-class="senda-builder-footer-cta" background-color="#111447" padding="38px 20px">
      <mj-column>
        <mj-text align="center" color="#8095ff" font-size="23px" line-height="1.25">Let us build the next experience together.</mj-text>
        <mj-button href="#" align="center" background-color="#5429ff" color="#ffffff" border-radius="24px">Schedule a meeting</mj-button>
        <mj-image src="https://placehold.co/170x40/png?text=Logo" alt="Logo" width="170px" padding-top="34px" />
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`

// createScreenshotFixture creates a throwaway template type + template and
// publishes a version, returning the template ID.  It mirrors the inline
// setup used by TestCRUD_WS_Template_PreviewMJML.
func createScreenshotFixture(t *testing.T, c *TestClient) string {
	return createScreenshotFixtureWithMJML(t, c, SampleMJML())
}

func createScreenshotFixtureWithMJML(t *testing.T, c *TestClient, bodyMJML string) string {
	t.Helper()

	ttSlug := fmt.Sprintf("ss-tt-%d", time.Now().UnixNano()%100000)
	resp := c.Post(wsPath()+"/template-types", TemplateTypeRequest{
		Slug: ttSlug,
		Name: "Screenshot Test Type",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var ttBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &ttBody)
	require.NotEmpty(t, ttBody.ID)

	resp2 := c.Post(wsPath()+"/templates", map[string]string{
		"template_type_id": ttBody.ID,
	})
	defer resp2.Body.Close()
	RequireStatus(t, resp2, http.StatusCreated)

	var tplBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp2, &tplBody)
	require.NotEmpty(t, tplBody.ID)

	path := mustWorkspacePath(TenantCode, WorkspaceCode)
	resp3 := c.Post(fmt.Sprintf("%s/templates/%s/versions", path, tplBody.ID), CreateVersionRequest{
		Subject:       TestSubject,
		PreviewText:   TestPreviewText,
		FromEmail:     TestFromEmail,
		FromName:      TestFromName,
		BodyMJML:      bodyMJML,
		DefaultLocale: "en",
	})
	defer resp3.Body.Close()
	RequireStatus(t, resp3, http.StatusCreated)

	var versionBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp3, &versionBody)
	require.NotEmpty(t, versionBody.ID)

	localeResp := c.Put(fmt.Sprintf("%s/templates/%s/versions/%s/locales/en", path, tplBody.ID, versionBody.ID), CreateLocaleRequest{
		Locale:      "en",
		Subject:     TestSubject,
		PreviewText: TestPreviewText,
		FromEmail:   TestFromEmail,
		FromName:    TestFromName,
		BodyMJML:    bodyMJML,
	})
	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, localeResp.StatusCode,
		"expected 200 or 409 setting locale, got %d: %s", localeResp.StatusCode, ReadResponseBody(t, localeResp))
	localeResp.Body.Close()

	publishResp := c.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", path, tplBody.ID, versionBody.ID), nil)
	require.Contains(t, []int{http.StatusNoContent, http.StatusConflict}, publishResp.StatusCode,
		"expected 204 or 409 publishing version, got %d: %s", publishResp.StatusCode, ReadResponseBody(t, publishResp))
	publishResp.Body.Close()

	return tplBody.ID
}

func requireScreenshotPNG(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	RequireStatus(t, resp, http.StatusOK)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"),
		"expected Content-Type image/png")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) >= 4 && body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4e && body[3] == 0x47,
		"response body does not start with PNG magic bytes")
	return body
}

// TestE2E_TemplateScreenshot_Desktop captures a desktop screenshot and asserts
// that the response is a valid PNG.  Requires SENDA_E2E_SCREENSHOT_ENABLED=true.
func TestE2E_TemplateScreenshot_Desktop(t *testing.T) {
	if !screenshotEnabled() {
		t.Skip("SENDA_E2E_SCREENSHOT_ENABLED not set; skipping screenshot smoke test")
	}

	c := ensureClient(t)
	templateID := createScreenshotFixture(t, c)

	resp := c.Get(fmt.Sprintf("%s/templates/%s/screenshot", wsPath(), templateID))
	defer resp.Body.Close()

	requireScreenshotPNG(t, resp)
}

// TestE2E_TemplateScreenshot_Mobile captures a mobile screenshot and asserts
// that the response is a valid PNG.  Requires SENDA_E2E_SCREENSHOT_ENABLED=true.
func TestE2E_TemplateScreenshot_Mobile(t *testing.T) {
	if !screenshotEnabled() {
		t.Skip("SENDA_E2E_SCREENSHOT_ENABLED not set; skipping screenshot smoke test")
	}

	c := ensureClient(t)
	templateID := createScreenshotFixture(t, c)

	resp := c.Get(fmt.Sprintf("%s/templates/%s/screenshot?viewport=mobile", wsPath(), templateID))
	defer resp.Body.Close()

	requireScreenshotPNG(t, resp)
}

func TestE2E_TemplateScreenshot_BuilderSectionBlocksDesktopAndMobile(t *testing.T) {
	if !screenshotEnabled() {
		t.Skip("SENDA_E2E_SCREENSHOT_ENABLED not set; skipping builder section screenshot test")
	}

	c := ensureClient(t)
	templateID := createScreenshotFixtureWithMJML(t, c, builderSectionBlocksScreenshotMJML)

	for _, tc := range []struct {
		name  string
		query string
	}{
		{name: "desktop", query: ""},
		{name: "mobile", query: "?viewport=mobile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.Get(fmt.Sprintf("%s/templates/%s/screenshot%s", wsPath(), templateID, tc.query))
			defer resp.Body.Close()
			require.NotEmpty(t, requireScreenshotPNG(t, resp))
		})
	}
}

// TestE2E_TemplateScreenshot_DisabledReturns503 verifies that the screenshot
// endpoint returns 503 SCREENSHOT_DISABLED when the feature is turned off.
// Skips when SENDA_E2E_SCREENSHOT_ENABLED=true (feature is on in that run).
func TestE2E_TemplateScreenshot_DisabledReturns503(t *testing.T) {
	if screenshotEnabled() {
		t.Skip("SENDA_E2E_SCREENSHOT_ENABLED is true; skipping disabled-feature assertion")
	}

	c := ensureClient(t)
	templateID := createScreenshotFixture(t, c)

	resp := c.Get(fmt.Sprintf("%s/templates/%s/screenshot", wsPath(), templateID))
	defer resp.Body.Close()

	RequireStatus(t, resp, http.StatusServiceUnavailable)

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	ParseJSONResponse(t, resp, &body)
	require.Equal(t, "SCREENSHOT_DISABLED", body.Code)
}
