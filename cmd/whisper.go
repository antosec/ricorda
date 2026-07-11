package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/antosec/ricorda/internal/analyze"
	"github.com/antosec/ricorda/internal/config"
	"github.com/antosec/ricorda/internal/index"
	"github.com/antosec/ricorda/internal/journal"
	"github.com/antosec/ricorda/internal/whisper"
	"github.com/spf13/cobra"
)

var (
	wExit   int
	wCmdB64 string
	wCWD    string
	wCWDB64 string
)

var whisperCmd = &cobra.Command{
	Use:   "whisper",
	Short: "Suggest your past victory right after a failure (used by the shell hooks)",
	Long: `Called synchronously by the hooks when a command exits non-zero. Reads
only the small victory index, so it never slows your prompt. One line,
your own past solution, then silence — with a per-tool cooldown.

Turn it off with: ricorda config whisper off`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if wExit == 0 || wCmdB64 == "" || !config.WhisperEnabled() {
			return nil
		}
		failed, err := journal.DecodeB64(wCmdB64)
		if err != nil {
			return nil // a broken payload must never surface in a prompt
		}
		failed = strings.Join(strings.Fields(failed), " ")
		tool := analyze.ToolOf(failed)
		if tool == "" {
			return nil
		}

		idx, err := index.Read()
		if err != nil {
			return nil
		}
		cwd := wCWD
		if wCWDB64 != "" {
			if d, err := journal.DecodeB64(wCWDB64); err == nil {
				cwd = d
			}
		}
		s := whisper.Pick(idx, tool, failed, cwd)
		if s == nil || whisper.OnCooldown(tool, time.Now()) {
			return nil
		}

		note := fmt.Sprintf("won after %d attempts", s.Attempts)
		if s.CostMS >= 1000 {
			note += ", " + analyze.FmtDurMS(s.CostMS)
		}
		line := fmt.Sprintf("ricorda remembers → %s   (%s)", s.Cmd, note)
		if os.Getenv("NO_COLOR") == "" {
			line = "\x1b[2m" + line + "\x1b[0m"
		}
		fmt.Fprintln(cmd.OutOrStdout(), line)
		return nil
	},
}

func init() {
	whisperCmd.Flags().IntVar(&wExit, "exit", 0, "exit code of the failed command")
	whisperCmd.Flags().StringVar(&wCmdB64, "cmd-b64", "", "base64-encoded failed command")
	whisperCmd.Flags().StringVar(&wCWD, "cwd", "", "working directory")
	whisperCmd.Flags().StringVar(&wCWDB64, "cwd-b64", "", "base64-encoded working directory (wins over --cwd)")
	rootCmd.AddCommand(whisperCmd)
}
