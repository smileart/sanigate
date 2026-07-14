package main

import (
	"strings"

	color "github.com/TwiN/go-color"
)

// bulletList renders items as one "• "-prefixed line each. Returns a placeholder
// rather than an empty string so a section never prints as a blank line.
func bulletList(items []string) string {
	if len(items) == 0 {
		return "• (none)"
	}
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("• ")
		b.WriteString(strings.TrimSpace(item))
	}
	return b.String()
}

// verdictLabel is the short headline for a verdict. The benign axis splits on
// capability so a powerful-but-legit installer reads distinctly from a plain safe
// script — that separation is the whole reason for the two-axis model.
func verdictLabel(v Verdict) string {
	switch v.Intent {
	case IntentMalicious:
		return "DANGEROUS"
	case IntentSuspicious:
		return "SUSPICIOUS"
	default: // benign
		if v.Capability == CapabilityHigh {
			return "LEGIT, BUT POWERFUL"
		}
		return "SAFE"
	}
}

// colorForVerdict colors text by the verdict: red = malicious, yellow = suspicious,
// cyan = benign-but-high-capability (legit installer — proceed with awareness),
// green = plainly safe. An unrecognized intent falls through to red (fail closed).
func colorForVerdict(v Verdict, text string) string {
	switch v.Intent {
	case IntentMalicious:
		return color.InRed(text)
	case IntentSuspicious:
		return color.InYellow(text)
	case IntentBenign:
		if v.Capability == CapabilityHigh {
			return color.InCyan(text)
		}
		return color.InGreen(text)
	default:
		return color.InRed(text)
	}
}
