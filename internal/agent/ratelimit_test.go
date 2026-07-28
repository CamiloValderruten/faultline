package agent

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsRateLimited(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated", err: errors.New("connection reset"), want: false},
		{
			name: "http 429",
			err:  fmt.Errorf("llm chat: %w", fmt.Errorf("chat completion: HTTP 429: Token Plan usage limit reached")),
			want: true,
		},
		{
			name: "rate limit wording",
			err:  errors.New("chat completion: rate limit exceeded"),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRateLimited(tc.err); got != tc.want {
				t.Fatalf("isRateLimited(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
