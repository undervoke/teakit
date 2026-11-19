// Command check-release validates the tag and release notes before publication.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: check-release TAG COMMIT")
		os.Exit(2)
	}
	tag, commit := os.Args[1], os.Args[2]
	if !regexp.MustCompile(`^v0\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(tag) {
		fmt.Fprintf(os.Stderr, "expected a Go version tag such as v0.1.0; got %q\n", tag)
		os.Exit(1)
	}
	path := "docs/releases/" + tag + ".md"
	cmd := exec.Command("git", "cat-file", "blob", commit+":"+path)
	cmd.Stderr = os.Stderr
	notes, err := cmd.Output()
	if err != nil || len(bytes.TrimSpace(notes)) == 0 {
		fmt.Fprintf(os.Stderr, "expected nonempty %s in commit %s\n", path, commit)
		os.Exit(1)
	}
}
