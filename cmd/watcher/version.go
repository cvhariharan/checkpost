package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build metadata, injected via -ldflags at build time:
//
//	-X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("watcher %s (commit %s, built %s)\n", version, commit, date)
			return nil
		},
	}
}
