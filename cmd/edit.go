package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/antosec/ricorda/internal/sheet"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <tool>",
	Short: "Open a cheatsheet in your editor (notes below the keep-marker survive rescans)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := sheet.PathFor(args[0])
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("no sheet for %q yet — run `ricorda scan` first", args[0])
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			if runtime.GOOS == "windows" {
				editor = "notepad"
			} else {
				editor = "vi"
			}
		}
		c := exec.Command(editor, path)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
