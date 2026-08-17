package main

import (
	"context"
	"os"

	"github.com/jon4hz/awoolt/cmd"
)

func main() {
	if err := cmd.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
