package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cdfuller/devhosts/internal/cli"
)

func main() {
	ctx := context.Background()
	if err := cli.Execute(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
