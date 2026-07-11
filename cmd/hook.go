package cmd

import (
	"fmt"

	"github.com/antosec/ricorda/internal/hook"
	"github.com/spf13/cobra"
)

var hookShell string

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage the shell hooks that turn guesses into ground truth",
	Long: `The hooks record every command's exit code, duration and directory
into the local journal (see: ricorda journal tail). With them, ricorda
stops guessing your struggles from history files and starts knowing.

Everything stays on this machine.`,
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the capture hook into your shell startup files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		targets := selectTargets()
		if len(targets) == 0 {
			return fmt.Errorf("no supported shell found (pwsh, bash, zsh, fish)")
		}
		installed := 0
		for _, t := range targets {
			if err := hook.InstallFile(t.Path, hook.BlockFor(t.Shell), t.OwnFile); err != nil {
				fmt.Fprintf(out, "  ✗ %-5s %s (%v)\n", t.Shell, t.Path, err)
				continue
			}
			fmt.Fprintf(out, "  ✓ %-5s %s\n", t.Shell, t.Path)
			installed++
		}
		if installed > 0 {
			fmt.Fprintln(out, "\nOpen a new shell (or re-source your profile) to start recording.")
			fmt.Fprintln(out, "Verify what gets stored with: ricorda journal tail")
			fmt.Fprintln(out, "Change your mind anytime with: ricorda hook uninstall")
		}
		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the capture hook from your shell startup files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		for _, t := range selectTargets() {
			removed, err := hook.UninstallFile(t.Path, t.OwnFile)
			switch {
			case err != nil:
				fmt.Fprintf(out, "  ✗ %-5s %s (%v)\n", t.Shell, t.Path, err)
			case removed:
				fmt.Fprintf(out, "  ✓ %-5s removed from %s\n", t.Shell, t.Path)
			default:
				fmt.Fprintf(out, "  - %-5s nothing to remove in %s\n", t.Shell, t.Path)
			}
		}
		fmt.Fprintln(out, "\nAlready-recorded data stays in your journal; delete it with your file manager if you want (see: ricorda journal path).")
		return nil
	},
}

var hookShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the exact block that install would add — inspect before you trust",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		shells := hook.Shells()
		if hookShell != "" {
			shells = []string{hookShell}
		}
		for _, s := range shells {
			b := hook.BlockFor(s)
			if b == "" {
				return fmt.Errorf("unknown shell %q (use one of: pwsh, bash, zsh, fish)", hookShell)
			}
			if len(shells) > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "# ---- %s ----\n", s)
			}
			fmt.Fprintln(cmd.OutOrStdout(), b)
		}
		return nil
	},
}

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show where the hook is installed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		for _, t := range selectTargets() {
			state := "not installed"
			if hook.Installed(t.Path) {
				state = "installed"
			}
			fmt.Fprintf(out, "  %-5s %-13s %s\n", t.Shell, state, t.Path)
		}
		return nil
	},
}

func selectTargets() []hook.Target {
	targets := hook.DetectTargets()
	if hookShell == "" {
		return targets
	}
	var out []hook.Target
	for _, t := range targets {
		if t.Shell == hookShell {
			out = append(out, t)
		}
	}
	return out
}

func init() {
	hookCmd.PersistentFlags().StringVar(&hookShell, "shell", "", "limit to one shell (pwsh, bash, zsh, fish)")
	hookCmd.AddCommand(hookInstallCmd, hookUninstallCmd, hookStatusCmd, hookShowCmd)
	rootCmd.AddCommand(hookCmd)
}
