package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"

	color "github.com/TwiN/go-color"
)

// version is the build's reported version. The default "0.4.0" is the
// in-source fallback; release builds override it via -ldflags "-X main.version=...".
var version = "0.4.0"

func main() {
	ver := flag.Bool("version", false, "print version and exit")
	porcelain := flag.Bool("porcelain", false, "do not do the plumbing after the analysis")
	porcelainShort := flag.Bool("p", false, "do not do the plumbing after the analysis (shorthand)")
	modelLong := flag.String("model", "", "OpenAI model to use (overrides SNGT_MODEL env var and config; default: gpt-4o-mini)")
	modelShort := flag.String("m", "", "OpenAI model to use (shorthand of --model)")

	flag.Parse()

	if *ver {
		fmt.Println(version)
		os.Exit(0)
	}

	isPorcelain := *porcelain || *porcelainShort

	flagModel := *modelLong
	if flagModel == "" {
		flagModel = *modelShort
	}

	// NOTE: All logs go to STDERR so we can output the original script to STDOUT
	errLog := log.New(os.Stderr, "", log.Lmsgprefix)

	cfgPath, err := configPath()
	if err != nil {
		errLog.Fatalf("Error: %v", err)
	}

	var cfg Config
	cfg, err = loadConfig(cfgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg = Config{}
			if writeErr := writeDefaultConfig(cfgPath); writeErr != nil {
				errLog.Printf("sanigate: failed to create default config at %s: %v", cfgPath, writeErr)
			} else {
				errLog.Printf("sanigate: created default config at %s — edit to customize", cfgPath)
			}
		} else {
			errLog.Fatalf("Error: %v", err)
		}
	}

	model := resolveModel(flagModel, os.Getenv(modelEnvVar), cfg.Model)
	apiKey := resolveAPIKey(os.Getenv(apiKeyEnvVar), cfg.APIKey)
	if apiKey == "" {
		errLog.Fatalf(
			"Error: no OpenAI API key found — set %s env var, or add `api_key = \"...\"` to %s",
			apiKeyEnvVar, cfgPath,
		)
	}

	// Preflight: warn (never block) if the model is known to lack a needed capability.
	checkModelCapabilities(errLog, model)

	client, ctx, err := initOpenAI(apiKey)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	printWarning(errLog)

	isPipe, err := isInputFromPipe()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if !isPipe {
		errLog.Fatal("Error: Expected input from pipe, but none was detected!")
	}

	script, err := readFromPipe()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if len(script) == 0 {
		log.Fatalf("Error: the script appears to be empty!")
	}

	chunks, err := sliceTextIntoChunks(script, model)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	var (
		allActions   []string
		allDangers   []string
		allRedFlags  []string
		chunkIntents []string
		chunkCaps    []string
	)

	for _, chunk := range chunks {
		analysis, err := analyzeChunk(client, ctx, makeChunkPrompt(chunk), model)
		if err != nil {
			errLog.Fatalf("Failed to analyze script: %v", err)
		}
		allActions = append(allActions, analysis.Actions...)
		allDangers = append(allDangers, analysis.Dangers...)
		allRedFlags = append(allRedFlags, analysis.RedFlags...)
		chunkIntents = append(chunkIntents, analysis.VerdictIntent)
		chunkCaps = append(chunkCaps, analysis.VerdictCapability)
	}

	conclusion, err := concludeAnalysis(
		client, ctx,
		makeConclusionPrompt(bulletList(allActions), bulletList(allRedFlags)),
		model,
	)
	if err != nil {
		errLog.Fatalf("Failed to summarize the script: %v", err)
	}

	// Fail closed on each axis: the whole-script judgment and the worst individual
	// chunk each get a vote. Intent takes the most-malicious value seen, capability
	// the highest — a rosy summary can't bury a chunk that already flagged malice.
	verdict := Verdict{
		Intent:     worstIntent(append(chunkIntents, conclusion.VerdictIntent)...),
		Capability: maxCapability(append(chunkCaps, conclusion.VerdictCapability)...),
	}

	// From here the process exits with an intent-keyed code. Fatalf paths above still
	// exit 1 (os.Exit skips deferred funcs), so an operational failure stays distinct
	// from a "dangerous" verdict. A benign-but-powerful installer exits 0.
	defer func() { os.Exit(verdict.exitCode()) }()

	errLog.Println(color.InBold("Actions:"))
	errLog.Println(bulletList(allActions))
	if len(allRedFlags) > 0 {
		errLog.Println(color.InBold("\nRed flags:"))
		errLog.Println(bulletList(allRedFlags))
	}
	// Danger: real risk factors even for a benign-but-powerful script (fetch+exec,
	// no checksum, root, …). Shown in yellow so the user weighs them before the
	// confirmation prompt, regardless of the intent verdict.
	if len(allDangers) > 0 {
		errLog.Println(color.InYellow(color.InBold("\nDanger:")))
		errLog.Println(color.InYellow(bulletList(allDangers)))
	}
	headline := fmt.Sprintf("%s (capability: %s)\n%s", verdictLabel(verdict), verdict.Capability, conclusion.Summary)
	errLog.Println("\n" + colorForVerdict(verdict, headline))

	// If no plumbing is needed, then exit
	if isPorcelain {
		return
	}

	// Ask the user if they want to pipe the original script to the next command
	confirm, err := askForConfirmation(errLog, "Do you want to redirect this script to STDOUT then? [y/N]: ")
	if err != nil {
		errLog.Printf("Failed to get confirmation: %v", err)
		return
	}
	if confirm {
		errLog.Println(
			color.InBold(
				color.InYellowOverBlack("WARNING: If you redirect it to sh/bash it will execute the script on your machine!"),
			),
		)

		confirmAgain, err := askForConfirmation(
			errLog,
			"Are you ABSOLUTELY positive that you want to pipe this script to the next command!? [y/N]: ",
		)
		if err != nil {
			errLog.Printf("Failed to get confirmation: %v", err)
			return
		}
		if confirmAgain {
			fmt.Println(script)
		}
	}
}
