package cmd

import (
	"fmt"

	"github.com/antosec/ricorda/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read or change ricorda's few settings",
}

var configWhisperCmd = &cobra.Command{
	Use:   "whisper [on|off]",
	Short: "Enable or disable post-failure suggestions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		if len(args) == 0 {
			state := "on"
			if !config.WhisperEnabled() {
				state = "off"
			}
			fmt.Fprintf(out, "whisper: %s\n", state)
			return nil
		}
		switch args[0] {
		case "on", "off":
			if err := config.Set("whisper", args[0]); err != nil {
				return err
			}
			fmt.Fprintf(out, "whisper: %s\n", args[0])
			return nil
		default:
			return fmt.Errorf("expected on or off, got %q", args[0])
		}
	},
}

func init() {
	configCmd.AddCommand(configWhisperCmd)
	rootCmd.AddCommand(configCmd)
}
