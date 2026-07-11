// Package project turns working directories into stable project identities,
// so a fight fought inside a repo stays attached to that repo — even when
// the same path is seen from Windows and from WSL.
package project

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var wslMount = regexp.MustCompile(`^/mnt/([a-z])(/|$)`)

// Normalize canonicalizes a path for matching across contexts: forward
// slashes, WSL drive mounts folded to Windows drives, trailing slash
// dropped, lowercased (Windows paths are case-insensitive; the rare
// case-only distinction on Linux is a fair trade for stable matching).
func Normalize(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if m := wslMount.FindStringSubmatch(p); m != nil {
		p = m[1] + ":" + p[len("/mnt/x"):]
	}
	p = strings.TrimSuffix(p, "/")
	return strings.ToLower(p)
}

// Root returns the enclosing project root of dir: the nearest ancestor
// containing a .git entry, or dir itself when none is found (or the path no
// longer exists — journal entries can outlive their directories).
func Root(dir string) string {
	if dir == "" {
		return ""
	}
	cur := filepath.Clean(dir)
	for i := 0; i < 64; i++ {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return filepath.Clean(dir)
}

// Label returns the human name for the project containing dir: the base
// name of its root. Safe for sheets — never the full path.
func Label(dir string) string {
	root := Root(dir)
	if root == "" {
		return ""
	}
	base := filepath.Base(filepath.ToSlash(root))
	if base == "/" || base == "." || strings.HasSuffix(base, ":") {
		return ""
	}
	return base
}

// Same reports whether two directories belong to the same project.
func Same(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return Normalize(Root(a)) == Normalize(Root(b))
}
