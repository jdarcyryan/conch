// Test is the conch test driver. It runs the same checks CI would —
// gofmt, go vet, and go test ./... — and reports a single line per
// stage so the output reads as a checklist rather than a wall of
// tool output. Stage failures stop the run with a non-zero exit
// code; tool stdout/stderr stream through unchanged for diagnosis.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// stage names a single check and the command that runs it. Custom
// reporting (e.g. gofmt's empty-output-means-pass) lives in run().
type stage struct {
	name string
	cmd  []string
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	stages := []stage{
		{"go vet", []string{"go", "vet", "./..."}},
		// gofmt -l prints offending files to stdout; our run() treats
		// any output as failure and surfaces the file list.
		{"gofmt -l", []string{"gofmt", "-l", "."}},
		{"go test", []string{"go", "test", "-count=1", "./..."}},
	}

	failed := 0
	for _, s := range stages {
		if err := run(s); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", s.name, err)
			failed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d stage(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Println("\nall stages passed")
}

// run executes one stage. Stdout and stderr are streamed through so
// failure output is immediately visible; for gofmt -l we additionally
// capture stdout because non-empty output (rather than a non-zero
// exit) signals failure.
func run(s stage) error {
	fmt.Printf("→ %s\n", s.name)
	start := time.Now()

	cmd := exec.Command(s.cmd[0], s.cmd[1:]...)

	switch s.name {
	case "gofmt -l":
		// Capture stdout so we can fail on any output. gofmt itself
		// always exits 0, so without this gate a misformatted file
		// would slip through the suite.
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		out := strings.TrimSpace(buf.String())
		if out != "" {
			fmt.Fprintln(os.Stderr, out)
			return fmt.Errorf("files need gofmt: %s", strings.ReplaceAll(out, "\n", ", "))
		}
	default:
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	fmt.Printf("  ✓ %s (%s)\n", s.name, time.Since(start).Round(time.Millisecond))
	return nil
}
