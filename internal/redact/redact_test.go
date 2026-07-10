package redact

import (
	"strings"
	"testing"
)

func TestCleanHidesSecrets(t *testing.T) {
	cases := []struct {
		in       string
		mustHide string
		mustKeep string
	}{
		{`mysql --password=hunter2 -u root`, "hunter2", "mysql"},
		{`vault login -token s3cr3ttoken`, "s3cr3ttoken", "vault login"},
		{`curl -H "Authorization: Bearer abc.def-123" https://api.example.com`, "abc.def-123", "https://api.example.com"},
		{`export GITHUB_TOKEN=ghp_abcdefghijklmnopqrstuv123456`, "ghp_", "export"},
		{`aws configure set key AKIAIOSFODNN7EXAMPLE`, "AKIAIOSFODNN7EXAMPLE", "aws configure"},
		{`stripe listen --api-key sk_test_abcdefghijklmnop123`, "sk_test_", "stripe listen"},
		{`docker login -u me -p sw0rdfish registry.example.com`, "", "registry.example.com"},
	}
	for _, c := range cases {
		got := Clean(c.in)
		if c.mustHide != "" && strings.Contains(got, c.mustHide) {
			t.Errorf("Clean(%q) = %q — still contains %q", c.in, got, c.mustHide)
		}
		if !strings.Contains(got, c.mustKeep) {
			t.Errorf("Clean(%q) = %q — lost %q", c.in, got, c.mustKeep)
		}
	}
}

func TestCleanLeavesNormalCommandsAlone(t *testing.T) {
	for _, in := range []string{
		`git commit -m "fix login flow"`,
		`docker run -it --rm ubuntu bash`,
		`ffmpeg -i in.mp4 -vf scale=640:-1 out.gif`,
		`git log --author "someone"`,
		`kubectl get pods -A`,
	} {
		if got := Clean(in); got != in {
			t.Errorf("Clean(%q) = %q, want unchanged", in, got)
		}
	}
}
