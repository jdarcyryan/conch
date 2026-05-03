package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const goWinresVersion = "v0.3.3"

type artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	step("Ensuring go-winres is installed")
	ensureTool("go-winres", "github.com/tc-hib/go-winres@"+goWinresVersion)

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

func sortOutput() {
	data, err := os.ReadFile(".output/artifacts.json")
	must(err)

	var arts []artifact
	must(json.Unmarshal(data, &arts))

	binParents := make(map[string]bool)

	for _, a := range arts {
		var dest string
		switch a.Type {
		case "Archive", "Linux Package", "Checksum":
			dest = ".output/release"
		case "Binary":
			dest = filepath.Join(".output/bin", filepath.Base(filepath.Dir(a.Path)))
			binParents[filepath.Dir(a.Path)] = true
		default:
			continue
		}
		moveTo(a.Path, dest)
	}

	for dir := range binParents {
		log.Printf("removing empty folder %s", dir)
		os.Remove(dir)
	}

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

func copyNativeBinary() {
	binDir := filepath.Join(".output/bin", "conch_"+runtime.GOOS+"_"+runtime.GOARCH+"_v1")
	if runtime.GOARCH == "arm64" {
		binDir = filepath.Join(".output/bin", "conch_"+runtime.GOOS+"_"+runtime.GOARCH+"_v8.0")
	}

	name := "conch"
	if runtime.GOOS == "windows" {
		name = "conch.exe"
	}

	src := filepath.Join(binDir, name)
	dest := filepath.Join(".output", name)

	log.Printf("copying %s → %s", src, dest)
	must(copyFile(src, dest))
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
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func ensureTool(binary, modulePath string) {
	if _, err := exec.LookPath(binary); err == nil {
		log.Printf("%s already installed", binary)
		return
	}
	log.Printf("installing %s", binary)
	run("go", "install", modulePath)
}

func run(name string, args ...string) {
	log.Printf("→ %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Run())
}

func step(msg string) {
	log.Printf("=== %s ===", msg)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
