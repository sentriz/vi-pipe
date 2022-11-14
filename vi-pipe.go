//go:build linux

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

const program = "vi-pipe"
const ttyPath = "/dev/tty"

func main() {
	log.SetFlags(0)
	reOpen := flag.Bool("re", false, "re-open editor, even your change has already been recorded")
	flag.Parse()

	editor, ok := os.LookupEnv("EDITOR")
	if !ok {
		log.Fatalf("$EDITOR not set")
	}
	if _, err := exec.LookPath(editor); err != nil {
		log.Fatalf("$EDITOR %q not found in $PATH", editor)
	}

	if err := run(os.Stdin, os.Stdout, editor, *reOpen); err != nil {
		log.Fatalf("running: %v", err)
	}
}

func run(inp io.Reader, out io.Writer, editor string, reOpen bool) error {
	inpBytes, err := io.ReadAll(inp)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	diffPath := fmt.Sprintf(".%s.diff", program)
	stat, err := os.Stat(diffPath)
	openEditor := reOpen || err != nil || time.Since(stat.ModTime()) > 30*time.Minute

	diffFile, err := os.OpenFile(diffPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open diff file: %w", err)
	}
	defer diffFile.Close()

	preDiff, err := io.ReadAll(diffFile)
	if err != nil {
		return fmt.Errorf("read diff: %w", err)
	}
	inpBytes = []byte(applyDiff(string(preDiff), string(inpBytes)))

	outBytes, err := editInput(editor, ttyPath, inpBytes, openEditor)
	if err != nil {
		return fmt.Errorf("edit input: %w", err)
	}

	postDiff := genDiff(string(inpBytes), string(outBytes))
	if _, err := diffFile.WriteString(postDiff); err != nil {
		return fmt.Errorf("write diff to file: %w", err)
	}
	if _, err := out.Write(outBytes); err != nil {
		return fmt.Errorf("write changes: %w", err)
	}
	return nil
}

func editInput(editor string, ttyPath string, inp []byte, withEditor bool) ([]byte, error) {
	if !withEditor {
		return inp, nil
	}

	tmpFile, err := os.CreateTemp("", filepath.Base(program))
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(inp); err != nil {
		return nil, fmt.Errorf("copy inp to tmp: %w", err)
	}

	// as we're in the middle of a pipeline, our normal stdin/stdout are not the parent's
	// where the editor needs to open. so open another pts and connect to that.
	tty, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("reopen tty: %w", err)
	}
	defer tty.Close()

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = tty
	cmd.Stdout = tty
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %q: %w", editor, err)
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek tmp file: %w", err)
	}
	out, err := io.ReadAll(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("read tmp: %w", err)
	}
	return out, nil
}

func genDiff(a, b string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a, b, false)
	patches := dmp.PatchMake(a, diffs)
	patchText := dmp.PatchToText(patches)
	return patchText
}

func applyDiff(diff string, a string) string {
	dmp := diffmatchpatch.New()
	patches, _ := dmp.PatchFromText(diff)
	b, _ := dmp.PatchApply(patches, a)
	return b
}
