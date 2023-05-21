package main

import (
	"fmt"
)

func makeListPrompt(chunk string) string {
	return fmt.Sprintf(
		`Given the following bash script, explain with bullet points what it's doing. Don't complete or print the script itself...

		[Script]
		%s

		[Format]
		Use "•" character in the list, do not give any title to the list itself. Do not leave empty lines. Put space in between the text of the point and the marker.`,
		chunk,
	)
}

func makeSecurityPrompt(chunk string) string {
	return fmt.Sprintf(
		`[Context]
		You are given a bash script that needs to be analyzed for security. Due to its size, the script is divided into smaller chunks. Please focus your analysis on the following chunk:

		[Chunk]
		%s

		[Question]
		Based on the provided chunk, please evaluate if it's secure to run the script on your machine. Please answer with one sentence only, starting with "It's ..."`,
		chunk,
	)
}

func makeConclusionPrompt(output string, securityAudit string) string {
	return fmt.Sprintf(
		`[Context]
		You've analysed the script split into parts and gave the following conclusions:
		"%s"
		Also you've checked what the script is doing and produced this list:
		"%s"

		[Task]
		Given this context and the list, summarize if it's safe to run such script on my computer.

		[Format]
		Give one sentence summary only, start it with "It's ..."`,
		securityAudit,
		output,
	)
}
