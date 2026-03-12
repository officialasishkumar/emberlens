package main

import (
	"os"

	"github.com/example/emberlens/internal/app"
)

func main() {
	r := app.Runner{Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(r.Run(os.Args[1:], os.Getenv("GITHUB_TOKEN")))
}
