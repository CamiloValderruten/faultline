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

func TestIsTransientLLMError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated auth", err: errors.New("HTTP 401: unauthorized"), want: false},
		{
			name: "connection reset",
			err:  errors.New("read tcp 172.20.0.2:42210->104.77.110.33:443: read: connection reset by peer"),
			want: true,
		},
		{
			name: "EOF",
			err:  errors.New("chat completion: Post \"https://api.minimax.io/v1/chat/completions\": EOF"),
			want: true,
		},
		{
			name: "502 Bad Gateway",
			err:  errors.New("chat completion: HTTP 502: Bad Gateway"),
			want: true,
		},
		{
			name: "503 Service Unavailable",
			err:  errors.New("chat completion: HTTP 503: Service Unavailable"),
			want: true,
		},
		{
			name: "timeout",
			err:  errors.New("context deadline exceeded"),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientLLMError(tc.err); got != tc.want {
				t.Fatalf("isTransientLLMError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
