package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	color "github.com/TwiN/go-color"
)

func printWarning(logger *log.Logger) {
	logger.Println(
		color.InBold(
			color.InYellowOverBlack(
				`
WARNING: This app relies on LLM and CANNOT serve as a replacement for a security audit.
         Use it at your own risk, and consider it merely an advisory tool!
				`,
			),
		),
	)
}

func readFromPipe() (string, error) {
	text := ""
	reader := bufio.NewReader(os.Stdin)
	scanner := bufio.NewScanner(bufio.NewReader(reader))
	for scanner.Scan() {
		text += scanner.Text() + "\n"
	}

	if scanner.Err() != nil {
		return "", scanner.Err()
	}

	return text, nil
}

func isInputFromPipe() (bool, error) {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	return fileInfo.Mode()&os.ModeCharDevice == 0, nil
}

// getUserInputFromTTY reads from /dev/tty instead of STDIN (cause STDIN is utilised by the input pipe)
func getUserInputFromTTY() (string, error) {
	file, err := os.Open("/dev/tty")
	if err != nil {
		return "", err
	}
	defer file.Close()

	userInputScanner := bufio.NewScanner(file)
	if userInputScanner.Scan() {
		return strings.Trim(userInputScanner.Text(), "\n"), nil
	}

	if err := userInputScanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}

func askForConfirmation(logger *log.Logger, question string) (bool, error) {
	logger.Print(question)
	answer, err := getUserInputFromTTY()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}
