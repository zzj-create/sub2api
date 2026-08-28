package service

import "strings"

// SanitizeStoredCredentials strips secrets that must never be persisted on the
// account credentials map after conversion to OAuth tokens. Grok OAuth is the
// deliberate exception for `sso`: the token is retained so the native quality
// guard can inspect account risk state through the account's bound proxy. DTO
// redaction still never returns the value to the admin UI.
// Call from admin create/update/import/apply-oauth paths.
//
// Cookie is always stripped: bulk paths may pass an empty platform label, and
// session-jar residue must never sit next to OAuth tokens on any platform.
// The platform argument is retained for call-site clarity / future scrubbing.
func SanitizeStoredCredentials(platform string, creds map[string]any) map[string]any {
	if creds == nil {
		return nil
	}
	sso := ""
	if platform == PlatformGrok {
		if value, ok := creds["sso"].(string); ok {
			sso = strings.TrimSpace(value)
		}
	}
	for _, key := range []string{
		"password", "sso_token", "sso", "sso-rw", "clearTextPassword", "cookie",
	} {
		delete(creds, key)
	}
	if platform == PlatformGrok && sso != "" {
		creds["sso"] = sso
	}
	return creds
}
