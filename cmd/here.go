package cmd

import (
	"fmt"
	"os"

	"github.com/antosec/ricorda/internal/analyze"
	"github.com/antosec/ricorda/internal/journal"
	"github.com/antosec/ricorda/internal/project"
	"github.com/spf13/cobra"
)

var hereCmd = &cobra.Command{
	Use:   "here",
	Short: "Show what you fought for in this project",
	Long: `Filters your certified fights down to the project you are standing in
(the enclosing git repository, or the current directory). Ground truth
comes from the shell hooks — install them with: ricorda hook install`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		jour, err := journal.ReadAll()
		if err != nil {
			return err
		}
		reports := analyze.Certify(nil, jour)

		label := project.Label(cwd)
		if label == "" {
			label = "this directory"
		}

		shown := 0
		for _, r := range reports {
			for _, h := range r.HardWon {
				if !project.Same(h.CWD, cwd) {
					continue
				}
				if shown == 0 {
					fmt.Fprintf(out, "Hard-won in %s:\n\n", label)
				}
				note := ""
				if h.Certified && h.CostMS >= 1000 {
					note = fmt.Sprintf("  (%s of fighting)", analyze.FmtDurMS(h.CostMS))
				}
				fmt.Fprintf(out, "  %-10s %s%s\n", r.Tool, h.Command, note)
				shown++
			}
		}
		if shown == 0 {
			fmt.Fprintf(out, "No certified fights recorded in %s yet.\n", label)
			fmt.Fprintln(out, "Ground truth needs the hooks: ricorda hook status")
			return nil
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(hereCmd)
}
