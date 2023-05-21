package main

import (
	"regexp"
)

var (
	regexEmptyLines    = regexp.MustCompile(`\n{2,}`)
	regexBulletMarkers = regexp.MustCompile(`(?is)•(\w)`)
	regexAnswerPrefix  = regexp.MustCompile(`(?is)(^\s+?\[?answer:?\]?|\s{2,})`)
)

func removeEmptyLines(input string) string {
	return regexEmptyLines.ReplaceAllString(input, "\n")
}

func fixBulletPoints(input string) string {
	return regexBulletMarkers.ReplaceAllString(input, "• $1")
}

func removeAnswerPrefix(input string) string {
	return regexAnswerPrefix.ReplaceAllString(input, "")
}
