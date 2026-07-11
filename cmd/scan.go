package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/antosec/ricorda/internal/analyze"
	"github.com/antosec/ricorda/internal/history"
	"github.com/antosec/ricorda/internal/journal"
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
			fmt.Fprintf(out, "  %-6s %6d commands  (%s)\n", s.Name, len(s.Entries), friendly(s.Path))
			entries = append(entries, s.Entries...)
		}
		jour, _ := journal.ReadAll()
		if len(jour) > 0 {
			fmt.Fprintf(out, "  %-6s %6d records   (ground truth: exit codes + timing)\n", "hooks", len(jour))
		}
		if found == 0 && len(jour) == 0 {
			return fmt.Errorf("no shell history found (looked for PowerShell, bash, zsh, fish)")
		}

		reports := analyze.Analyze(entries)
		reports = analyze.Certify(reports, jour)

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
		fmt.Fprintf(out, "Wrote %d cheatsheet(s) → %s\n", written, friendly(dir))

		// Rank fights by the single worst battle: HardWon is sorted by
		// attempts, so index 0 is each tool's hardest command.
		var fights []analyze.ToolReport
		for _, r := range reports {
			if len(r.HardWon) == 0 {
				continue
			}
			if _, err := sheet.PathFor(r.Tool); err == nil {
				fights = append(fights, r)
			}
		}
		sort.Slice(fights, func(i, j int) bool {
			return fights[i].HardWon[0].Attempts > fights[j].HardWon[0].Attempts
		})
		if len(fights) > 0 {
			fmt.Fprintf(out, "\nYour biggest fights:\n")
			for i, r := range fights {
				if i >= 3 {
					break
				}
				top := r.HardWon[0]
				note := ""
				if top.Certified && top.CostMS > 0 {
					note = fmt.Sprintf("  (%s lost)", analyze.FmtDurMS(top.CostMS))
				}
				fmt.Fprintf(out, "  %-14s %d attempts before it worked%s\n", r.Tool, top.Attempts, note)
			}
			fmt.Fprintf(out, "\nTry: ricorda %s\n", fights[0].Tool)
		} else if first := firstTool(reports); first != "" {
			fmt.Fprintf(out, "\nTry: ricorda %s\n", first)
		}
		return nil
	},
}

// friendly shortens a path for display: the home directory becomes ~ and
// separators are normalized. Keeps output tidy and avoids echoing usernames.
func friendly(p string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(p, home); ok {
			p = "~" + rest
		}
	}
	return filepath.ToSlash(p)
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
