package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootFlags holds the persistent flags shared by the client subcommands
// (apply). They are distinct from the server's koanf TOML config: --config
// here points at the CLI YAML config file for client commands, and at the
// server TOML for `watcher server`.
type rootFlags struct {
	server   string
	token    string
	config   string
	insecure bool
}

func newRootCmd() *cobra.Command {
	flags := &rootFlags{}

	serverCmd := newServerCmd(flags)

	root := &cobra.Command{
		Use:   "watcher",
		Short: "Watcher detection platform server and GitOps CLI",
		Long: "Watcher is an osquery-based detection platform.\n\n" +
			"Run `watcher server` to start the HTTP server (the default when no\n" +
			"subcommand is given), or `watcher apply` to push YAML-defined detection\n" +
			"content to a running server using an API token.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Default to `server` so existing bare-binary invocations keep working.
		RunE: serverCmd.RunE,
	}

	root.PersistentFlags().StringVar(&flags.server, "server", "", "Watcher server base URL (client commands; env WATCHER_SERVER)")
	root.PersistentFlags().StringVar(&flags.token, "token", "", "API token for client commands (env WATCHER_TOKEN)")
	root.PersistentFlags().StringVar(&flags.config, "config", "", "Path to the config file (server TOML, or CLI YAML for client commands)")
	root.PersistentFlags().BoolVar(&flags.insecure, "insecure", false, "Skip TLS certificate verification and allow sending the token to a non-loopback plain-http host")

	root.AddCommand(serverCmd)
	root.AddCommand(newApplyCmd(flags))
	root.AddCommand(newVersionCmd())

	return root
}

// Execute runs the root command and sets the process exit code on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
