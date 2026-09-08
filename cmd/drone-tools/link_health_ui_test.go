package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestLinkHealthUI(t *testing.T) {
	content, err := fs.ReadFile(webFiles, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	for _, marker := range []string{"health-link-score", "health-link-status", "h.link?.available", "not available"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("missing link health UI marker %q", marker)
		}
	}
}
