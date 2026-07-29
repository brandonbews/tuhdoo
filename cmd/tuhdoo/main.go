package main

import (
	"fmt"
	"os"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("tuhdoo " + version)
		return
	}
	fmt.Fprintln(os.Stderr, "tuhdoo "+version+" — no commands implemented yet; see docs/plan/backlog.md")
	os.Exit(1)
}
