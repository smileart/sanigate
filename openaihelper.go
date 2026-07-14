package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

const (
	chunkSize = 1500
	maxTokens = 2000

	// structuredTemperature is 0 so a given script always yields the same verdict.
	structuredTemperature = 0
)

// The verdict is split across two orthogonal axes so that a legitimate installer
// (high blast radius, no malice) reads differently from actual malware (evidence of
// harm). Conflating them onto one "how risky" scale is what makes every `curl | sh`
// installer look as dangerous as an obfuscated payload.
//
//	intent     — is there EVIDENCE OF MALICE? benign / suspicious / malicious
//	capability — BLAST RADIUS if it wanted to do harm: low / medium / high
//
// An installer is benign intent with high capability. Malware is malicious intent.
const (
	IntentBenign     = "benign"
	IntentSuspicious = "suspicious"
	IntentMalicious  = "malicious"

	CapabilityLow    = "low"
	CapabilityMedium = "medium"
	CapabilityHigh   = "high"
)

// ChunkAnalysis is GPT's structured verdict on a single chunk of the script.
//
// Field order matters: the model emits JSON in schema order, and the schema is
// serialized with keys sorted alphabetically (jsonschema.Definition.Properties is a
// map). The names are chosen so evidence sorts before the verdict — actions,
// red_flags, reason, then the two verdict_* fields last — so the model enumerates
// what it sees and only then commits to a judgment. Renaming these can silently
// reorder them; check the resulting sort order if you do.
type ChunkAnalysis struct {
	Actions           []string `json:"actions"            description:"Each distinct action this chunk performs, in plain language."`
	Dangers           []string `json:"dangers"            description:"Concrete risk factors the user should understand before running this, EVEN IF the script is benign: e.g. downloads and executes a binary or script without verifying a checksum/signature, requires or uses root/sudo, pipes remote content straight into a shell, makes irreversible or system-wide changes, or depends on a third-party server staying trustworthy. These are cautions about a legitimate-but-powerful script, distinct from evidence of malice. Empty array if genuinely none."`
	RedFlags          []string `json:"red_flags"          description:"Concrete evidence of MALICIOUS intent only: obfuscation (base64/hex/eval of code that is then executed), exfiltration of secrets/keys/credentials, disabling or tampering with security tools, hidden persistence (cron, udev, LaunchAgents, shell rc), destructive commands (rm -rf /, dd, shred, mkfs), or actions that contradict the script's stated purpose. Empty array if there are none — downloading and running an official binary is NOT a red flag."`
	Reason            string   `json:"reason"             description:"One sentence explaining the intent and capability ratings for this chunk."`
	VerdictCapability string   `json:"verdict_capability" enum:"low,medium,high"           description:"Blast radius if this chunk were malicious: low = read-only/limited; medium = writes files or installs to user space; high = root, network fetch-and-execute, or system-wide changes."`
	VerdictIntent     string   `json:"verdict_intent"     enum:"benign,suspicious,malicious" description:"Evidence of malice. benign = legitimate even if powerful (an installer that downloads and runs an official binary is benign despite high capability). suspicious = unusual/risky patterns that could be malicious but are not clearly so. malicious = red flags present; evidence it is designed to harm, deceive, or exfiltrate."`
}

// Conclusion is GPT's cross-chunk verdict on the script as a whole. It exists
// because chunks that are individually harmless can compose into something that is
// not (fetch here, chmod there, execute elsewhere). "summary" sorts before the two
// verdict_* fields, keeping prose before the judgment.
type Conclusion struct {
	Summary           string `json:"summary"            description:"One sentence summarizing what the whole script does."`
	VerdictCapability string `json:"verdict_capability" enum:"low,medium,high"           description:"Overall blast radius of the script."`
	VerdictIntent     string `json:"verdict_intent"     enum:"benign,suspicious,malicious" description:"Overall intent of the script. Downloading and executing an official installer is benign; reserve malicious for evidence of harm, deception, or exfiltration."`
}

var (
	chunkAnalysisSchema = mustSchema(ChunkAnalysis{})
	conclusionSchema    = mustSchema(Conclusion{})
)

// mustSchema reflects a JSON schema from v. It panics rather than returning an
// error because failure depends only on the shape of the Go type, never on user
// input — a broken schema fails at startup for everyone, so it cannot ship.
func mustSchema(v any) *jsonschema.Definition {
	def, err := jsonschema.GenerateSchemaForType(v)
	if err != nil {
		panic(fmt.Sprintf("sanigate: cannot build JSON schema for %T: %v", v, err))
	}
	return def
}

var (
	intentOrder     = map[string]int{IntentBenign: 0, IntentSuspicious: 1, IntentMalicious: 2}
	capabilityOrder = map[string]int{CapabilityLow: 0, CapabilityMedium: 1, CapabilityHigh: 2}
)

// worstIntent returns the most malicious of the given intents. An unrecognized or
// empty value is treated as malicious: if we cannot understand the verdict, we do
// not get to call the script benign.
func worstIntent(intents ...string) string {
	worst := IntentBenign
	for _, in := range intents {
		rank, ok := intentOrder[in]
		if !ok {
			return IntentMalicious
		}
		if rank > intentOrder[worst] {
			worst = in
		}
	}
	return worst
}

// maxCapability returns the highest blast radius of the given capabilities. An
// unrecognized or empty value is treated as high, matching the fail-closed stance.
func maxCapability(caps ...string) string {
	max := CapabilityLow
	for _, c := range caps {
		rank, ok := capabilityOrder[c]
		if !ok {
			return CapabilityHigh
		}
		if rank > capabilityOrder[max] {
			max = c
		}
	}
	return max
}

// Process exit codes keyed to intent, so callers can gate a pipeline on malice
// (e.g. `sanigate -p script && run-it`). Capability does NOT raise the exit code: a
// benign-but-powerful installer exits 0, which is the whole point of the two axes.
// These sit clear of 1 (operational error) and 2 (flag-parse usage).
const (
	exitSafe       = 0
	exitSuspicious = 3
	exitDangerous  = 4
)

// Verdict is the aggregated two-axis judgment for the whole script.
type Verdict struct {
	Intent     string
	Capability string
}

// exitCode maps intent to the process exit code. Capability is intentionally
// ignored here — being powerful is not being malicious.
func (v Verdict) exitCode() int {
	switch v.Intent {
	case IntentMalicious:
		return exitDangerous
	case IntentSuspicious:
		return exitSuspicious
	default:
		return exitSafe
	}
}

// initOpenAI initializes the OpenAI client with the provided API key.
func initOpenAI(apiKey string) (*openai.Client, context.Context, error) {
	if apiKey == "" {
		return nil, nil, fmt.Errorf("API key is empty")
	}
	client := openai.NewClient(apiKey)
	ctx := context.Background()
	return client, ctx, nil
}

// getTokenSize returns the number of tokens in prompt for the given model.
func getTokenSize(model string, prompt string) (int, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding for model: %w", err)
	}

	token := tkm.Encode(prompt, nil, nil)
	return len(token), nil
}

// sliceTextIntoChunks splits the input text into chunks of size chunkSize
// (measured in OpenAI tokens for the given model). Falls back to cl100k_base
// if the model is unknown to tiktoken-go.
func sliceTextIntoChunks(text, model string) ([]string, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sanigate: tiktoken doesn't recognize %q, falling back to cl100k_base for chunking\n", model)
		tkm, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, fmt.Errorf("tiktoken fallback failed: %w", err)
		}
	}

	lines := strings.Split(text, "\n")
	chunks := []string{""}
	chunkIndex := 0

	for _, line := range lines {
		ll := len(tkm.Encode(line, nil, nil))
		cl := len(tkm.Encode(chunks[chunkIndex], nil, nil))

		if !(cl+ll < chunkSize) {
			chunkIndex++
			chunks = append(chunks, "")
		}

		chunks[chunkIndex] += line + "\n"
	}

	return chunks, nil
}

// requestStructured sends prompt to model and decodes the reply into T, which the
// API is constrained to produce by schema (OpenAI Structured Outputs, strict mode).
// schemaName is the name reported to the API; it appears in API-side errors.
//
// Every failure path returns an error rather than a zero T: an empty verdict would
// read as neither "safe" nor "dangerous", and silently degrading to "not dangerous"
// is the one bug this tool cannot afford.
func requestStructured[T any](
	client *openai.Client,
	ctx context.Context,
	prompt, model, schemaName string,
	schema *jsonschema.Definition,
) (T, error) {
	var out T

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
		MaxTokens:   maxTokens,
		Temperature: structuredTemperature,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   schemaName,
				Schema: schema,
				Strict: true,
			},
		},
	})
	if err != nil {
		return out, fmt.Errorf("completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return out, fmt.Errorf("model %q returned no choices", model)
	}
	choice := resp.Choices[0]

	// The scripts this tool analyzes are frequently malicious by design, so the
	// model refusing to answer is a normal outcome, not an anomaly.
	if choice.Message.Refusal != "" {
		return out, fmt.Errorf("model refused to analyze the script: %s", choice.Message.Refusal)
	}

	// A truncated reply is invalid JSON, so catch it here rather than letting it
	// surface as an inscrutable parse error.
	if choice.FinishReason == openai.FinishReasonLength {
		return out, fmt.Errorf("model hit the %d-token limit before completing its answer", maxTokens)
	}

	// Unmarshal via the schema so a well-formed reply carrying a bogus enum value
	// is rejected too, not just malformed JSON.
	if err := schema.Unmarshal(choice.Message.Content, &out); err != nil {
		return out, fmt.Errorf("could not decode %s: %w", schemaName, err)
	}

	return out, nil
}

// analyzeChunk asks the model what a single chunk of the script does and how risky it is.
func analyzeChunk(client *openai.Client, ctx context.Context, prompt, model string) (ChunkAnalysis, error) {
	return requestStructured[ChunkAnalysis](client, ctx, prompt, model, "chunk_analysis", chunkAnalysisSchema)
}

// concludeAnalysis asks the model for a verdict on the script as a whole.
func concludeAnalysis(client *openai.Client, ctx context.Context, prompt, model string) (Conclusion, error) {
	return requestStructured[Conclusion](client, ctx, prompt, model, "conclusion", conclusionSchema)
}
