package utils

import "regexp"

var scrubRegexp = regexp.MustCompile(
	`(?im)([\?&]((?:private|authenticity|rss)[\-_]token)|secret|passw|X-AMZ-Signature|X-AMZ-Credential)=[^& ]*`,
)

// ScrubSecrets replaces the content of any sensitive query string parameters in a URL with `[FILTERED]`
func ScrubSecrets(url string) string {
	return scrubRegexp.ReplaceAllString(url, "$1=[FILTERED]")
}
