package main

import (
	"fmt"
)

// makeChunkPrompt asks the model to enumerate a chunk's actions, surface red flags,
// and rate it on two separate axes. Most formatting lives in the JSON schema; the
// prompt's job is to hammer the one calibration that matters: high capability is not
// the same as malice, so ordinary installers must not be rated malicious.
func makeChunkPrompt(chunk string) string {
	return fmt.Sprintf(
		`You are a security reviewer analyzing one chunk of a larger bash script.
Analyze only the chunk below, on its own. Do not reproduce the script.

Rate it on two SEPARATE axes:

  • capability (blast radius): how much could this affect the system if it wanted to?
    Downloading and running software, needing root, or touching the whole system all
    push capability up. This is about power, not wrongdoing.

  • intent (evidence of malice): is there anything actually BAD here? Crucially, a
    script that downloads and runs an official installer or binary is BENIGN even
    though its capability is high — that is how legitimate installers work, and high
    capability alone is never malicious. Only rate above benign when you see real
    evidence: obfuscation, exfiltration of secrets, security-tool tampering, hidden
    persistence, destructive commands, or behavior that contradicts the stated purpose.

Record any such evidence in red_flags (leave it empty when there is none).

Separately, in dangers, list the concrete risk factors a user should weigh before
running this EVEN IF it is benign — e.g. it downloads and executes a binary without
verifying a checksum, needs root, pipes remote content into a shell, or makes
irreversible system-wide changes. A legitimate installer usually still has real
dangers; surface them so the user can make an informed choice.

[Chunk]
%s`,
		chunk,
	)
}

// makeConclusionPrompt asks for a whole-script judgment on the same two axes, given
// the actions and red flags gathered from every chunk.
func makeConclusionPrompt(actions string, redFlags string) string {
	return fmt.Sprintf(
		`You are a security reviewer. A bash script was split into chunks and each was
analyzed. Below are the combined actions and the combined red flags from all chunks.

Judge the script AS A WHOLE on two separate axes — capability (blast radius) and
intent (evidence of malice). Keep these in mind:

  • Downloading and executing an official binary is how legitimate installers work:
    that is benign intent with high capability, not malicious.
  • Reserve "malicious" for actual evidence of harm, deception, or exfiltration —
    see the red flags below.
  • Individually harmless chunks can combine into something harmful, so weigh the
    whole picture, not just isolated lines.

Summarize what the script does in one sentence.

[Actions]
%s

[Red flags]
%s`,
		actions,
		redFlags,
	)
}
