package mjml

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

const videoThumbnailPath = "/public/video-thumbnail"

var (
	mjImageTagRe   = regexp.MustCompile(`(?is)<mj-image\b[^>]*\/?>`)
	srcAttrRe      = regexp.MustCompile(`\bsrc\s*=\s*"([^"]*)"`)
	cssClassAttrRe = regexp.MustCompile(`\bcss-class\s*=\s*"([^"]*)"`)
)

func rewriteVideoThumbnailMJML(mjmlContent, baseURL string) string {
	return mjImageTagRe.ReplaceAllStringFunc(mjmlContent, func(tag string) string {
		cssClassMatch := cssClassAttrRe.FindStringSubmatch(tag)
		if len(cssClassMatch) != 2 || !hasVideoCSSClass(cssClassMatch[1]) {
			return tag
		}

		srcMatch := srcAttrRe.FindStringSubmatch(tag)
		if len(srcMatch) != 2 {
			return tag
		}

		rewritten := buildVideoThumbnailURL(srcMatch[1], baseURL)
		if rewritten == srcMatch[1] {
			return tag
		}

		return srcAttrRe.ReplaceAllString(tag, `src="`+rewritten+`"`)
	})
}

func hasVideoCSSClass(raw string) bool {
	for _, className := range strings.Fields(raw) {
		if className == "senda-video" {
			return true
		}
	}
	return false
}

func buildVideoThumbnailURL(src, baseURL string) string {
	original := extractOriginalThumbnailURL(src)
	if strings.TrimSpace(original) == "" {
		return src
	}

	if strings.HasPrefix(original, "data:") {
		return original
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return original
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return original
	}

	parsedBase.Path = path.Join(parsedBase.Path, videoThumbnailPath)
	parsedBase.RawQuery = url.Values{"url": []string{original}}.Encode()
	return parsedBase.String()
}

func extractOriginalThumbnailURL(src string) string {
	if src == "" {
		return ""
	}

	parsed, err := url.Parse(src)
	if err == nil && parsed.Path == videoThumbnailPath {
		if raw := parsed.Query().Get("url"); raw != "" {
			return raw
		}
	}

	if strings.Contains(src, videoThumbnailPath+"?url=") {
		idx := strings.Index(src, "?url=")
		raw := src[idx+5:]
		decoded, err := url.QueryUnescape(raw)
		if err == nil {
			return decoded
		}
		return raw
	}

	return src
}
