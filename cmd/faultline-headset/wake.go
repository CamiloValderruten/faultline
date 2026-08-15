package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const wakeFrameSamples = 1280 // 80 ms at 16 kHz (openWakeWord chunk)

func stripWakePrefix(text string) string {
	s := strings.TrimSpace(text)
	lower := strings.ToLower(s)
	for _, p := range []string{"hey alexa", "ok alexa", "alexa"} {
		if strings.HasPrefix(lower, p) {
			rest := strings.TrimSpace(s[len(p):])
			return strings.TrimLeft(rest, ",.!?;: ")
		}
	}
	return s
}

type waker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func startWaker(ctx context.Context) (*waker, error) {
	script := envOr("HEADSET_WAKE_PY", "/usr/local/bin/headset-wake.py")
	cmd := exec.CommandContext(ctx, "python3", "-u", script)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("wake detector: %w", err)
	}
	return &waker{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil
}

func (w *waker) feed(frame []byte) (bool, error) {
	if _, err := w.stdin.Write(frame); err != nil {
		return false, err
	}
	line, err := w.stdout.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(line) == "WAKE", nil
}

func (w *waker) close() {
	if w == nil {
		return
	}
	_ = w.stdin.Close()
	if w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
	}
}
