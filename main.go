package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	color "github.com/TwiN/go-color"
)

func main() {
	const unsafeKeyword = "not safe"
	const notSecure = "not secure"
	version := "0.3.0"

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

	output := ""
	securityAudit := ""

	for i, chunk := range chunks {
		listPrompt := makeListPrompt(chunk)
		securityPrompt := makeSecurityPrompt(chunk)

		// Ask GPT to make a list of bullet points about chunk actions
		resp, err := requestCompletion(client, ctx, listPrompt, model)
		if err != nil {
			errLog.Fatalf("Failed to request completion: %v", err)
		}
		output += "\n" + resp

		// Ask GPT to make a security audit of a chunk
		resp, err = requestCompletion(client, ctx, securityPrompt, model)
		if err != nil {
			errLog.Fatalf("Failed to request completion: %v", err)
		}
		securityAudit += resp + "\n"

		// If it's the last chunk, then ask GPT to make a summary
		if i == len(chunks)-1 {
			output = removeEmptyLines(output)
			output = fixBulletPoints(output)

			conclusionPrompt := makeConclusionPrompt(output, securityAudit)
			resp, err = requestCompletion(client, ctx, conclusionPrompt, model)
			if err != nil {
				errLog.Fatalf("Failed to request completion: %v", err)
			}

			if len(resp) == 0 {
				continue
			}

			resp = removeAnswerPrefix(resp)

			if strings.Contains(resp, unsafeKeyword) || strings.Contains(resp, notSecure) {
				resp = color.InRed(resp)
			} else {
				resp = color.InGreen(resp)
			}

			output += "\n\n" + strings.TrimSpace(resp)
		}
	}

	// Print the actions list
	errLog.Println(output)

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
