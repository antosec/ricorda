package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/antosec/ricorda/internal/journal"
	"github.com/spf13/cobra"
)

var journalCmd = &cobra.Command{
	Use:   "journal",
	Short: "Inspect or feed the ground-truth command journal",
	Long: `The journal is where the shell hooks record every command with its
exit code, duration and directory — redacted first, stored locally,
never leaving the machine. Install the hooks with: ricorda hook install`,
}

var (
	jShell  string
	jExit   int
	jDurMS  int64
	jCWD    string
	jCWDB64 string
	jCmdB64 string
	jStdin  bool
)

var journalAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Record one command outcome (used by the shell hooks)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var text string
		switch {
		case jCmdB64 != "":
			decoded, err := journal.DecodeB64(jCmdB64)
			if err != nil {
				return err
			}
			text = decoded
		case jStdin:
			b, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024))
			if err != nil {
				return err
			}
			text = string(b)
		default:
			return fmt.Errorf("provide the command via --cmd-b64 or --stdin")
		}
		cwd := jCWD
		if jCWDB64 != "" {
			if decoded, err := journal.DecodeB64(jCWDB64); err == nil {
				cwd = decoded
			}
		}
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		return journal.Append(journal.Entry{
			Cmd:   text,
			Exit:  jExit,
			DurMS: jDurMS,
			CWD:   cwd,
			Shell: jShell,
		})
	},
}

var journalTailN int

var journalTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show the latest recorded entries — see exactly what is stored",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := journal.ReadAll()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Journal is empty. Install the hooks with: ricorda hook install")
			return nil
		}
		start := len(entries) - journalTailN
		if start < 0 {
			start = 0
		}
		for _, e := range entries[start:] {
			status := "ok"
			if e.Exit != 0 {
				status = fmt.Sprintf("exit %d", e.Exit)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %-8s %-7s %6dms  %s\n",
				e.TS.Format("2006-01-02 15:04:05"), e.Shell, status, e.DurMS, e.Cmd)
		}
		return nil
	},
}

var journalPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the journal directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := journal.Dir()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), d)
		return nil
	},
}

func init() {
	journalAddCmd.Flags().StringVar(&jShell, "shell", "", "shell that ran the command (pwsh, bash, zsh, fish)")
	journalAddCmd.Flags().IntVar(&jExit, "exit", 0, "exit code of the command")
	journalAddCmd.Flags().Int64Var(&jDurMS, "dur-ms", 0, "duration in milliseconds")
	journalAddCmd.Flags().StringVar(&jCWD, "cwd", "", "working directory (defaults to current)")
	journalAddCmd.Flags().StringVar(&jCWDB64, "cwd-b64", "", "base64-encoded working directory (wins over --cwd)")
	journalAddCmd.Flags().StringVar(&jCmdB64, "cmd-b64", "", "base64-encoded command text")
	journalAddCmd.Flags().BoolVar(&jStdin, "stdin", false, "read command text from stdin")

	journalTailCmd.Flags().IntVarP(&journalTailN, "lines", "n", 10, "number of entries to show")

	journalCmd.AddCommand(journalAddCmd, journalTailCmd, journalPathCmd)
	rootCmd.AddCommand(journalCmd)
}
