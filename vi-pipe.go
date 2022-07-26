package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sergi/go-diff/diffmatchpatch"
)

const program = "vi-pipe"
const ttyPath = "/dev/tty"

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatalf("please provide a <key>")
	}

	editor, ok := os.LookupEnv("EDITOR")
	if !ok {
		log.Fatalf("$EDITOR not set")
	}
	if _, err := exec.LookPath(editor); err != nil {
		log.Fatalf("$EDITOR %q not found in $PATH", editor)
	}

	if err := run(os.Stdin, os.Stdout, editor, os.Args[1]); err != nil {
		log.Fatalf("running: %v", err)
	}
}

func run(inp io.Reader, out io.Writer, editor string, diffKey string) error {
	inpBytes, err := io.ReadAll(inp)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	diffPath, err := diffPath(diffKey)
	if err != nil {
		return fmt.Errorf("gen diff path: %w", err)
	}
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

	tmpFile, err := os.CreateTemp("", filepath.Base(program))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	outBytes, err := editInput(tmpFile, editor, ttyPath, inpBytes)
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

func editInput(tmp *os.File, editor string, ttyPath string, inp []byte) ([]byte, error) {
	if _, err := tmp.Write(inp); err != nil {
		return nil, fmt.Errorf("copy inp to tmp: %w", err)
	}

	// as we're in the middle of a pipeline, our normal stdin/stdout are not the parent's
	// where the editor needs to open. so open another pts and connect to that.
	newTTY, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("reopen tty: %w", err)
	}
	defer newTTY.Close()

	cmd := exec.Command(editor, tmp.Name())
	cmd.Stdin = newTTY
	cmd.Stdout = newTTY
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %q: %w", editor, err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek tmp file: %w", err)
	}

	out, err := io.ReadAll(tmp)
	if err != nil {
		return nil, fmt.Errorf("read tmp: %w", err)
	}
	return out, nil
}

func diffPath(key string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache dir: %w", err)
	}
	hash := sha1.New()
	hash.Write([]byte(key))
	keyPathPrefixed := fmt.Sprintf("%s.%x", program, hash.Sum(nil))
	return filepath.Join(cacheDir, keyPathPrefixed), nil
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
