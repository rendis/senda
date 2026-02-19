package tracking

import (
	"strings"
	"testing"
)

func TestTransparentGIF_IsValid(t *testing.T) {
	if len(TransparentGIF) == 0 {
		t.Fatal("TransparentGIF is empty")
	}
	// Must start with GIF89a header.
	if string(TransparentGIF[:6]) != "GIF89a" {
		t.Errorf("invalid GIF header: %x", TransparentGIF[:6])
	}
	// Must end with trailer byte.
	if TransparentGIF[len(TransparentGIF)-1] != 0x3b {
		t.Error("GIF missing trailer byte 0x3b")
	}
}

func TestInjectOpenPixel(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		baseURL    string
		trackingID string
		wantPixel  bool
	}{
		{
			name:       "injects before </body>",
			html:       `<html><body><p>Hello</p></body></html>`,
			baseURL:    "https://track.example.com",
			trackingID: "trk_abc123",
			wantPixel:  true,
		},
		{
			name:       "no </body> tag",
			html:       `<html><p>Hello</p></html>`,
			baseURL:    "https://track.example.com",
			trackingID: "trk_abc123",
			wantPixel:  false,
		},
		{
			name:       "empty base URL",
			html:       `<html><body><p>Hello</p></body></html>`,
			baseURL:    "",
			trackingID: "trk_abc123",
			wantPixel:  false,
		},
		{
			name:       "empty tracking ID",
			html:       `<html><body><p>Hello</p></body></html>`,
			baseURL:    "https://track.example.com",
			trackingID: "",
			wantPixel:  false,
		},
		{
			name:       "uppercase BODY tag",
			html:       `<html><BODY><p>Hello</p></BODY></html>`,
			baseURL:    "https://track.example.com",
			trackingID: "trk_upper",
			wantPixel:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InjectOpenPixel(tt.html, tt.baseURL, tt.trackingID)

			hasPixel := strings.Contains(result, `/t/o/`+tt.trackingID)
			if hasPixel != tt.wantPixel {
				t.Errorf("pixel present = %v, want %v\nresult: %s", hasPixel, tt.wantPixel, result)
			}

			if tt.wantPixel {
				// Pixel must appear BEFORE </body>.
				pixelIdx := strings.Index(result, `/t/o/`+tt.trackingID)
				bodyIdx := strings.LastIndex(strings.ToLower(result), "</body>")
				if pixelIdx > bodyIdx {
					t.Error("pixel injected after </body>")
				}
			}
		})
	}
}
