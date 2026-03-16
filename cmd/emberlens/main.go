package main

import (
	"os"

	"github.com/officialasishkumar/emberlens/internal/app"
)

func main() {
	runner := app.NewRunner(os.Stdout, os.Stderr)
	code := runner.Run(os.Args[1:], os.Getenv("GITHUB_TOKEN"), os.Getenv("GITLAB_TOKEN"))
	os.Exit(code)
}
