package coverimage

import (
	"fmt"
	"net/url"
	"strings"
)

const DirectUnsplashGuidance = "Use a real direct Unsplash CDN URL copied from an image result, in the form https://images.unsplash.com/photo-1562869929-bda0650edb1f?ixid=...&ixlib=rb-4.1.0. The path after photo- must start with numeric timestamp digits and a hyphen; do not use unsplash.com/photos/... or short image IDs such as https://images.unsplash.com/photo-nWdsya5_Yms."

func ValidateDirectUnsplashURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("cover_image_url must be a valid URL. %s", DirectUnsplashGuidance)
	}
	if parsed.Scheme != "https" || parsed.Host != "images.unsplash.com" {
		return fmt.Errorf("cover_image_url must start with https://images.unsplash.com/. %s", DirectUnsplashGuidance)
	}

	const prefix = "/photo-"
	if !strings.HasPrefix(parsed.EscapedPath(), prefix) {
		return fmt.Errorf("cover_image_url must use an images.unsplash.com/photo-<numeric timestamp>-<asset hash> asset path. %s", DirectUnsplashGuidance)
	}

	assetID := strings.TrimPrefix(parsed.EscapedPath(), prefix)
	digits, suffix, ok := strings.Cut(assetID, "-")
	if !ok || len(digits) < 10 || suffix == "" || !allASCIIDigits(digits) || !allASCIILettersOrDigits(suffix) {
		return fmt.Errorf("cover_image_url must use a numeric Unsplash photo asset path such as photo-1562869929-bda0650edb1f, not a short photo ID. %s", DirectUnsplashGuidance)
	}
	return nil
}

func allASCIIDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func allASCIILettersOrDigits(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return value != ""
}
