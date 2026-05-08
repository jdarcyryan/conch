// Build is the conch release driver. It installs go-winres if needed,
// generates Windows resource files, runs goreleaser in snapshot mode,
// and lays out the resulting artefacts under .output/.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const goWinresVersion = "v0.3.3"

// artifact mirrors the subset of goreleaser's artifacts.json we use.
type artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	step("Ensuring go-winres is installed")
	ensureTool("go-winres", "github.com/tc-hib/go-winres@"+goWinresVersion)

	step("Cleaning up .syso files")
	cleanSyso()

	step("Generating Windows resources")
	run("go-winres", "make", "--in", "winres/winres.json")

	step("Running goreleaser")
	run("goreleaser", "release", "--snapshot", "--clean")

	step("Cleaning up .syso files")
	cleanSyso()

	step("Sorting output artifacts")
	sortOutput()

	step("Copying native binary to root")
	copyNativeBinary()

	step("Done")
}

// sortOutput reorganises goreleaser's flat .output/ tree into
// release/, bin/<target>/, and choco/conch.
func sortOutput() {
	data, err := os.ReadFile(".output/artifacts.json")
	must(err)

	var arts []artifact
	must(json.Unmarshal(data, &arts))

	binParents := map[string]struct{}{}
	for _, a := range arts {
		var dest string
		switch a.Type {
		case "Archive", "Linux Package", "Checksum":
			dest = ".output/release"
		case "Binary":
			dest = filepath.Join(".output/bin", filepath.Base(filepath.Dir(a.Path)))
			binParents[filepath.Dir(a.Path)] = struct{}{}
		default:
			continue
		}
		moveTo(a.Path, dest)
	}

	// Drop the now-empty per-target source directories goreleaser left behind.
	for dir := range binParents {
		log.Printf("removing empty folder %s", dir)
		os.Remove(dir)
	}

	// .nupkg files aren't listed in artifacts.json — sweep them up by glob.
	nupkgs, _ := filepath.Glob(".output/*.nupkg")
	for _, f := range nupkgs {
		moveTo(f, ".output/release")
	}

	if _, err := os.Stat(".output/conch.choco"); err == nil {
		dest := ".output/choco/conch"
		log.Printf("moving .output/conch.choco → %s", dest)
		must(os.MkdirAll(filepath.Dir(dest), 0o755))
		must(os.Rename(".output/conch.choco", dest))
	}
}

// copyNativeBinary lifts the binary built for the current host out of
// its goreleaser target directory into .output/ for ease of local use.
func copyNativeBinary() {
	src := filepath.Join(".output/bin", nativeTargetDir(), nativeBinaryName())
	dest := filepath.Join(".output", nativeBinaryName())

	log.Printf("copying %s → %s", src, dest)
	must(copyFile(src, dest))
}

// nativeTargetDir returns the goreleaser-style folder name for the
// current host, e.g. conch_linux_amd64_v1 or conch_windows_arm64_v8.0.
func nativeTargetDir() string {
	variant := "v1"
	if runtime.GOARCH == "arm64" {
		variant = "v8.0"
	}
	return fmt.Sprintf("conch_%s_%s_%s", runtime.GOOS, runtime.GOARCH, variant)
}

func nativeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "conch.exe"
	}
	return "conch"
}

func cleanSyso() {
	matches, _ := filepath.Glob("*.syso")
	for _, f := range matches {
		log.Printf("removing %s", f)
		os.Remove(f)
	}
}

func moveTo(src, destDir string) {
	must(os.MkdirAll(destDir, 0o755))
	dest := filepath.Join(destDir, filepath.Base(src))
	log.Printf("moving %s → %s", src, dest)
	must(os.Rename(src, dest))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dest, err)
	}
	return nil
}

// ensureTool installs binary from modulePath via `go install` if the
// binary is not already discoverable on PATH.
func ensureTool(binary, modulePath string) {
	if _, err := exec.LookPath(binary); err == nil {
		log.Printf("%s already installed", binary)
		return
	}
	log.Printf("installing %s", binary)
	run("go", "install", modulePath)
}

// run executes name with args, streams output to the parent process,
// and aborts the build on failure.
func run(name string, args ...string) {
	log.Printf("→ %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Run())
}

func step(msg string) { log.Printf("=== %s ===", msg) }

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
