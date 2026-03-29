package tracking

import (
	"fmt"
	"regexp"
)

// TransparentGIF is a minimal 1x1 transparent GIF (43 bytes).
var TransparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // GIF89a
	0x01, 0x00, 0x01, 0x00, // 1x1
	0x80, 0x00, 0x00, // GCT flag, 2 colors
	0xff, 0xff, 0xff, // color 0: white
	0x00, 0x00, 0x00, // color 1: black
	0x21, 0xf9, 0x04, // GCE
	0x01, 0x00, 0x00, 0x00, 0x00, // transparent index 0
	0x2c, // image descriptor
	0x00, 0x00, 0x00, 0x00, // left, top
	0x01, 0x00, 0x01, 0x00, // width, height
	0x00,       // no LCT
	0x02,       // LZW min code size
	0x02,       // block size
	0x4c, 0x01, // compressed data
	0x00, // block terminator
	0x3b, // trailer
}

// bodyCloseRe matches </body> case-insensitively using byte offsets that are
// safe for multi-byte UTF-8 strings (unlike strings.ToLower + LastIndex).
var bodyCloseRe = regexp.MustCompile(`(?i)</body>`)

// InjectOpenPixel inserts an open-tracking <img> tag before </body> in the
// compiled HTML. If there is no </body> tag the HTML is returned unchanged.
func InjectOpenPixel(html, baseURL, trackingID string) string {
	if baseURL == "" || trackingID == "" {
		return html
	}

	loc := bodyCloseRe.FindStringIndex(html)
	if loc == nil {
		return html // no </body> found, return as-is
	}
	idx := loc[0]

	pixel := fmt.Sprintf(`<img src="%s/t/o/%s" width="1" height="1" alt="" style="display:none" />`, baseURL, trackingID)
	return html[:idx] + pixel + html[idx:]
}
