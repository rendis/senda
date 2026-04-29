package unsubscribe

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// PeekWorkspaceID extracts the workspace UUID from a token without verifying
// its HMAC signature. The caller MUST then call Verify with the workspace's
// signing key. Returns ok=false if the token is malformed.
func PeekWorkspaceID(token string) (uuid.UUID, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return uuid.Nil, false
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	var w struct {
		WS uuid.UUID `json:"ws"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return uuid.Nil, false
	}
	if w.WS == uuid.Nil {
		return uuid.Nil, false
	}
	return w.WS, true
}
