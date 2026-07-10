package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at release time via -ldflags "-X github.com/antosec/ricorda/cmd.version=v0.x.y".
var version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "ricorda [tool]",
	Short: "The cheatsheet that writes itself — from your own shell history",
	Long: `ricorda (Italian for "remember!") reads your shell history locally,
finds the commands you fought for, and turns them into personal,
per-tool markdown cheatsheets.

Everything runs on your machine. Nothing ever leaves it.`,
	Example: `  ricorda scan      # read your history and (re)build your sheets
  ricorda           # list your sheets
  ricorda docker    # show your personal docker sheet`,
	Args:          cobra.MaximumNArgs(1),
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runList(cmd.OutOrStdout())
		}
		return runShow(cmd.OutOrStdout(), args[0])
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ricorda:", err)
		os.Exit(1)
	}
}
