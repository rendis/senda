package request

// TemplateScreenshotQuery models the GET screenshot query string.
type TemplateScreenshotQuery struct {
	Viewport  string `query:"viewport"`
	VersionID string `query:"version_id"`
	Locale    string `query:"locale"`
}
