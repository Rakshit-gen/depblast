package main

import "testing"

func TestRealName(t *testing.T) {
	for _, c := range []struct{ key, resolved, want string }{
		{"typescript", "https://registry.npmjs.org/@typescript/typescript6/-/typescript6-6.0.2.tgz", "@typescript/typescript6"},
		{"ms", "https://registry.npmjs.org/ms/-/ms-2.0.0.tgz", "ms"},
		{"native-preview", "file:packages/vscode-typescript", "native-preview"},
		{"left-pad", "", "left-pad"},
	} {
		if got := realName(c.key, c.resolved); got != c.want {
			t.Errorf("realName(%q, %q) = %q, want %q", c.key, c.resolved, got, c.want)
		}
	}
}
