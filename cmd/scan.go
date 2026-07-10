package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/antosec/ricorda/internal/analyze"
	"github.com/antosec/ricorda/internal/history"
	"github.com/antosec/ricorda/internal/sheet"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Read your shell history and (re)generate your cheatsheets",
	Long: `Reads every shell history it can find (PowerShell, bash, zsh, fish),
detects the commands you struggled with, and writes one markdown
cheatsheet per tool. Anything that looks like a secret is redacted
before it touches disk.

Only aggregate numbers are printed; your commands stay in the sheets.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		sources := history.Load()
		var entries []history.Entry
		found := 0
		for _, s := range sources {
			if s.Err != nil {
				continue
			}
			found++
			fmt.Fprintf(out, "  %-6s %6d commands  (%s)\n", s.Name, len(s.Entries), s.Path)
			entries = append(entries, s.Entries...)
		}
		if found == 0 {
			return fmt.Errorf("no shell history found (looked for PowerShell, bash, zsh, fish)")
		}

		reports := analyze.Analyze(entries)

		written := 0
		when := time.Now().Format("2006-01-02")
		for _, r := range reports {
			if len(r.HardWon) == 0 && len(r.Hits) == 0 {
				continue
			}
			if _, err := sheet.PathFor(r.Tool); err != nil {
				continue // a name we can't store safely (parse artifact)
			}
			if _, err := sheet.Write(r, when); err != nil {
				return err
			}
			written++
		}

		dir, err := sheet.SheetsDir()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "\nScanned %d commands across %d source(s).\n", len(entries), found)
		fmt.Fprintf(out, "Wrote %d cheatsheet(s) → %s\n", written, dir)

		top := make([]analyze.ToolReport, len(reports))
		copy(top, reports)
		sort.Slice(top, func(i, j int) bool { return len(top[i].HardWon) > len(top[j].HardWon) })
		shown := 0
		for _, r := range top {
			if len(r.HardWon) == 0 || shown >= 3 {
				break
			}
			if shown == 0 {
				fmt.Fprintf(out, "\nYour biggest fights:\n")
			}
			fmt.Fprintf(out, "  %-14s %d hard-won command(s)\n", r.Tool, len(r.HardWon))
			shown++
		}
		if first := firstTool(reports); first != "" {
			fmt.Fprintf(out, "\nTry: ricorda %s\n", first)
		}
		return nil
	},
}

func firstTool(reports []analyze.ToolReport) string {
	for _, r := range reports {
		if len(r.HardWon) == 0 && len(r.Hits) == 0 {
			continue
		}
		if _, err := sheet.PathFor(r.Tool); err == nil {
			return r.Tool
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
