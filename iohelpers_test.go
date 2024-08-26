package main

// Tests for the iohelpers.go file
import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	_ "syscall"
	"testing"

	color "github.com/TwiN/go-color"
)

func Test_printWarning(t *testing.T) {
	r, w, _ := os.Pipe()

	// restore stdout right after the test
	defer func(v *os.File) { os.Stderr = v }(os.Stderr)
	os.Stderr = w
	logger := log.New(os.Stderr, "", log.Lmsgprefix)
	printWarning(logger)
	_ = w.Close()
	out, _ := ioutil.ReadAll(r)
	strOut := string(out)

	if strings.Contains(strOut, color.InBold(color.InYellowOverBlack("WARNING"))) {
		t.Errorf("printWarning() did not print the warning message!")
	}
}

func Test_readFromPipe(t *testing.T) {
	input := []byte("Test input\nfrom pipe")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Write(input)
	if err != nil {
		t.Error(err)
	}
	w.Close()

	// Restore stdin right after the test
	defer func(v *os.File) { os.Stdin = v }(os.Stdin)
	os.Stdin = r

	text, err := readFromPipe()
	if text != string(input)+"\n" {
		t.Log([]byte(text))
		t.Log(input)
		t.Errorf("readFromPipe() = '%v', want '%v'", text, string(input)+"\n")
	}
}
