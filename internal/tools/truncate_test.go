package tools

import (
	"strings"
	"testing"
)

func TestTruncateToolResult(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		in := "hello"
		if got := truncateToolResult(in, 0); got != in {
			t.Fatalf("got %q", got)
		}
		if got := truncateToolResult(in, -1); got != in {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("under limit", func(t *testing.T) {
		in := "hello"
		if got := truncateToolResult(in, 100); got != in {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("over limit", func(t *testing.T) {
		in := "abcdefghij"
		got := truncateToolResult(in, 5)
		if !strings.HasPrefix(got, "abcde") {
			t.Fatalf("prefix = %q", got)
		}
		if !strings.Contains(got, "[truncated:") || !strings.Contains(got, "5 of 10") {
			t.Fatalf("missing truncation marker: %q", got)
		}
	})
}
