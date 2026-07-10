// Package redact scrubs likely secrets from commands before they are
// written anywhere.
package redact

import "regexp"

const placeholder = "«redacted»"

type rule struct {
	re   *regexp.Regexp
	repl string
}

var rules = []rule{
	// --password foo, --token=bar, -api-key baz …
	{regexp.MustCompile(`(?i)(^|\s)(--?(?:password|passwd|pwd|token|secret|api[-_]?key|apikey|access[-_]?key|auth)[= ])\S+`), "${1}${2}" + placeholder},
	// FOO_TOKEN=xxx, MY_SECRET=yyy, PASSWORD=zzz …
	{regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASSWD|API_?KEY)[A-Z0-9_]*=)\S+`), "${1}" + placeholder},
	// Authorization: Bearer xxx (as passed to curl -H and friends)
	{regexp.MustCompile(`(?i)(authorization:\s*(?:bearer|basic)\s+)[A-Za-z0-9._~+/=-]+`), "${1}" + placeholder},
	// Well-known credential shapes: GitHub, Slack, AWS, Stripe, JWT.
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), placeholder},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), placeholder},
	{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), placeholder},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), placeholder},
	{regexp.MustCompile(`\b(?:sk|pk|rk)_(?:live|test)_[A-Za-z0-9]{16,}\b`), placeholder},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\b`), placeholder},
}

// Clean replaces likely secrets in a command with a placeholder. It is
// deliberately eager: a slightly over-redacted cheatsheet is still useful,
// a leaked credential is not.
func Clean(cmd string) string {
	for _, r := range rules {
		cmd = r.re.ReplaceAllString(cmd, r.repl)
	}
	return cmd
}
