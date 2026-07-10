package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/antosec/ricorda/internal/sheet"
)

func runList(out io.Writer) error {
	metas, err := sheet.List()
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		fmt.Fprintln(out, "No cheatsheets yet. Run `ricorda scan` first.")
		return nil
	}
	fmt.Fprintf(out, "%d cheatsheet(s):\n\n", len(metas))
	for _, m := range metas {
		fmt.Fprintf(out, "  %-16s updated %s\n", m.Tool, m.ModTime.Format("2006-01-02"))
	}
	fmt.Fprintln(out, "\nShow one with: ricorda <tool>")
	return nil
}

func runShow(out io.Writer, tool string) error {
	content, err := sheet.Read(tool)
	if err != nil {
		return fmt.Errorf("no sheet for %q — run `ricorda scan` first, or `ricorda` to list what exists", tool)
	}
	fmt.Fprint(out, colorize(content))
	return nil
}

// colorize adds minimal ANSI styling to markdown headings unless NO_COLOR is set.
func colorize(md string) string {
	if os.Getenv("NO_COLOR") != "" {
		return md
	}
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ") {
			lines[i] = "\x1b[1m" + l + "\x1b[0m"
		}
	}
	return strings.Join(lines, "\n")
}
