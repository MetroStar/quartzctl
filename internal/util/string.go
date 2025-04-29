package util

import (
	"regexp"
	"strings"
)

// TruncateString truncates a string to the specified length `n`.
// If the string is shorter than or equal to `n`, it is returned unchanged.
func TruncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[0:n]
}

// TruncateStringE truncates a string to the specified length `n` and appends "..." if truncation occurs.
// If `n` is less than or equal to 3, the string is truncated without appending ellipses.
func TruncateStringE(s string, n int) string {
	if len(s) <= n {
		return s
	}

	if n <= 3 { // can't add ellipses
		return TruncateString(s, n)
	}

	return s[0:n-3] + "..."
}

// EqualsOrRegexMatchString checks if a string `s` matches a pattern strictly or via a regex.
// If `insensitive` is true, the match is case-insensitive.
func EqualsOrRegexMatchString(pattern string, s string, insensitive bool) bool {
	if pattern == s {
		return true
	}

	if insensitive {
		if strings.EqualFold(pattern, s) {
			return true
		}

		if !strings.HasPrefix(pattern, "(?i)") {
			pattern = "(?i)" + pattern
		}
	}

	if match, _ := regexp.MatchString(pattern, s); match {
		return true
	}

	return false
}
