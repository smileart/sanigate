package main

// Tests for the two-axis verdict logic (openaihelper.go) and its rendering
// (render.go). These are the security-load-bearing pure functions: the fail-closed
// branches in particular must not regress, since "unknown → treat as safe" is the
// one mistake this tool cannot make.

import (
	"strings"
	"testing"
)

func Test_worstIntent(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"empty is benign", nil, IntentBenign},
		{"single benign", []string{IntentBenign}, IntentBenign},
		{"malicious beats benign", []string{IntentBenign, IntentMalicious, IntentBenign}, IntentMalicious},
		{"suspicious beats benign", []string{IntentBenign, IntentSuspicious}, IntentSuspicious},
		{"malicious beats suspicious", []string{IntentSuspicious, IntentMalicious}, IntentMalicious},
		{"fail closed: unknown is malicious", []string{IntentBenign, "wat"}, IntentMalicious},
		{"fail closed: empty string is malicious", []string{IntentBenign, ""}, IntentMalicious},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := worstIntent(c.in...); got != c.want {
				t.Errorf("worstIntent(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func Test_maxCapability(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"empty is low", nil, CapabilityLow},
		{"high beats medium", []string{CapabilityMedium, CapabilityHigh, CapabilityLow}, CapabilityHigh},
		{"medium beats low", []string{CapabilityLow, CapabilityMedium}, CapabilityMedium},
		{"fail closed: unknown is high", []string{CapabilityLow, "huge"}, CapabilityHigh},
		{"fail closed: empty string is high", []string{CapabilityLow, ""}, CapabilityHigh},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maxCapability(c.in...); got != c.want {
				t.Errorf("maxCapability(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func Test_Verdict_exitCode(t *testing.T) {
	cases := []struct {
		intent string
		want   int
	}{
		{IntentBenign, exitSafe},
		{IntentSuspicious, exitSuspicious},
		{IntentMalicious, exitDangerous},
	}
	for _, c := range cases {
		t.Run(c.intent, func(t *testing.T) {
			// Capability must never change the exit code — a benign-but-powerful
			// installer exits 0, which is the whole point of the two axes.
			for _, cap := range []string{CapabilityLow, CapabilityMedium, CapabilityHigh} {
				v := Verdict{Intent: c.intent, Capability: cap}
				if got := v.exitCode(); got != c.want {
					t.Errorf("Verdict{%q,%q}.exitCode() = %d, want %d", c.intent, cap, got, c.want)
				}
			}
		})
	}
}

func Test_verdictLabel(t *testing.T) {
	cases := []struct {
		intent string
		cap    string
		want   string
	}{
		{IntentMalicious, CapabilityHigh, "DANGEROUS"},
		{IntentMalicious, CapabilityLow, "DANGEROUS"},
		{IntentSuspicious, CapabilityHigh, "SUSPICIOUS"},
		{IntentBenign, CapabilityLow, "SAFE"},
		{IntentBenign, CapabilityMedium, "SAFE"},
		// The distinguishing case: benign but high capability is the legit installer.
		{IntentBenign, CapabilityHigh, "LEGIT, BUT POWERFUL"},
	}
	for _, c := range cases {
		t.Run(c.intent+"/"+c.cap, func(t *testing.T) {
			if got := verdictLabel(Verdict{Intent: c.intent, Capability: c.cap}); got != c.want {
				t.Errorf("verdictLabel(%q,%q) = %q, want %q", c.intent, c.cap, got, c.want)
			}
		})
	}
}

func Test_colorForVerdict(t *testing.T) {
	// We don't assert exact escape codes (that couples to the color lib); instead we
	// check the text survives and that the four visual states are mutually distinct,
	// so a benign installer never renders identically to malware.
	text := "verdict text"
	states := map[string]Verdict{
		"malicious":  {IntentMalicious, CapabilityHigh},
		"suspicious": {IntentSuspicious, CapabilityHigh},
		"powerful":   {IntentBenign, CapabilityHigh},
		"safe":       {IntentBenign, CapabilityLow},
	}
	seen := map[string]string{}
	for name, v := range states {
		out := colorForVerdict(v, text)
		if !strings.Contains(out, text) {
			t.Errorf("colorForVerdict(%s) dropped the text: %q", name, out)
		}
		for other, prev := range seen {
			if prev == out {
				t.Errorf("colorForVerdict(%s) renders identically to %s", name, other)
			}
		}
		seen[name] = out
	}
}

func Test_bulletList(t *testing.T) {
	if got := bulletList(nil); got != "• (none)" {
		t.Errorf("bulletList(nil) = %q, want %q", got, "• (none)")
	}
	if got := bulletList([]string{"a"}); got != "• a" {
		t.Errorf("bulletList([a]) = %q, want %q", got, "• a")
	}
	got := bulletList([]string{"first", "  second  "})
	want := "• first\n• second"
	if got != want {
		t.Errorf("bulletList trims and joins: got %q, want %q", got, want)
	}
}
