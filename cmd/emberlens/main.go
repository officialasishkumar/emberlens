package main

import (
	"os"

	"github.com/officialasishkumar/emberlens/internal/app"
)

func main() {
	r := app.NewRunner(os.Stdout, os.Stderr)
	os.Exit(r.Run(os.Args[1:], os.Getenv("GITHUB_TOKEN"), os.Getenv("GITLAB_TOKEN")))
}
