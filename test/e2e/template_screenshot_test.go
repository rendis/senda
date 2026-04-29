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

// createScreenshotFixture creates a throwaway template type + template and
// publishes a version, returning the template ID.  It mirrors the inline
// setup used by TestCRUD_WS_Template_PreviewMJML.
func createScreenshotFixture(t *testing.T, c *TestClient) string {
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

	MustEnsureVersionPublished(t, c, TenantCode, WorkspaceCode, tplBody.ID)

	return tplBody.ID
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

	RequireStatus(t, resp, http.StatusOK)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"),
		"expected Content-Type image/png")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) >= 4 && body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4e && body[3] == 0x47,
		"response body does not start with PNG magic bytes")
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

	RequireStatus(t, resp, http.StatusOK)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"),
		"expected Content-Type image/png")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) >= 4 && body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4e && body[3] == 0x47,
		"response body does not start with PNG magic bytes")
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
