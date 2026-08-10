package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	labelDaemon          = "faultline.daemon"
	labelOwner           = "faultline.owner"
	labelDaemonID        = "faultline.daemon.id"
	labelDaemonName      = "faultline.daemon.name"
	labelDaemonDesc      = "faultline.daemon.description"
	labelDaemonCreatedAt = "faultline.daemon.created_at"

	daemonOwnerFile           = ".daemon-owner"
	maxDaemonDescriptionRunes = 512
	defaultDaemonMax          = 5
)

// DaemonInfo is one long-lived daemon container owned by this sandbox.
type DaemonInfo struct {
	ID          string `json:"daemon_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Container   string `json:"container"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// EnableDaemons loads or creates a stable owner UUID under the sandbox
// dir and sets the max concurrent daemon cap. Call once at startup when
// [daemons] is enabled.
func (s *Sandbox) EnableDaemons(max int) error {
	if max <= 0 {
		max = defaultDaemonMax
	}
	owner, err := s.ensureDaemonOwner()
	if err != nil {
		return err
	}
	s.daemonOwner = owner
	s.daemonMax = max
	s.startDaemonAlertWatch()
	return nil
}

// DaemonOwner returns the stable owner UUID, or "" when daemons are disabled.
func (s *Sandbox) DaemonOwner() string { return s.daemonOwner }

func (s *Sandbox) ensureDaemonOwner() (string, error) {
	path := filepath.Join(s.dir, daemonOwnerFile)
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", daemonOwnerFile, err)
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate daemon owner id: %w", err)
	}
	// ponytail: UUID v4-ish string without importing a UUID package
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	id := fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", daemonOwnerFile, err)
	}
	return id, nil
}

// daemonRunArgs builds `docker run -d` args for a persistent daemon.
// No --rm: the container must survive stop/reboot for label rediscovery
// until intentionally removed by StopDaemon.
func (s *Sandbox) daemonRunArgs(containerName, owner, id, name, description, createdAt string, env map[string]string, command []string) []string {
	workHost := filepath.Join(s.dir, "daemons", id)
	args := []string{
		"run", "-d",
		"--restart", "unless-stopped",
		"--name", containerName,
		"--label", labelDaemon + "=1",
		"--label", labelOwner + "=" + owner,
		"--label", labelDaemonID + "=" + id,
		"--label", labelDaemonName + "=" + name,
		"--label", labelDaemonDesc + "=" + description,
		"--label", labelDaemonCreatedAt + "=" + createdAt,
		"-v", filepath.Join(s.dir, "scripts") + ":/scripts:ro",
		"-v", filepath.Join(s.dir, "input") + ":/input:ro",
		"-v", filepath.Join(s.dir, "output") + ":/output:rw",
		"-v", filepath.Join(s.dir, "venv") + ":/venv:rw",
		"-v", filepath.Join(s.dir, "node") + ":/node:rw",
		"-v", filepath.Join(s.dir, "cache") + ":/cache:rw",
		"-v", filepath.Join(s.dir, "pyproject.toml") + ":/pyproject.toml:rw",
		"-v", filepath.Join(s.dir, "uv.lock") + ":/uv.lock:rw",
		"-v", workHost + ":/work:rw",
		"-e", "HOME=/cache/home",
		"-e", "npm_config_cache=/cache/npm",
		"-e", "XDG_CACHE_HOME=/cache",
		"-e", "UV_CACHE_DIR=/cache",
		"-e", "UV_LINK_MODE=copy",
		"-e", "UV_PROJECT_ENVIRONMENT=/venv",
		"-e", "PATH=/node/node_modules/.bin:/usr/local/bin:/usr/bin:/bin",
		"-w", "/",
		"--memory", s.memoryLimit,
		"--user", fmt.Sprintf("%d:%d", s.uid, s.gid),
		"--security-opt", "no-new-privileges",
	}
	args = appendEnvFlags(args, s.env)
	args = appendEnvFlags(args, env)
	args = append(args, "-e", alertsEnvVar+"="+alertsPathInCtr)
	if !s.network {
		args = append(args, "--network=none")
	}
	args = append(args, s.image)
	args = append(args, command...)
	return args
}

// StartDaemon starts a detached container that survives Faultline and
// host restarts (--restart unless-stopped). command must reference a
// flat file under /scripts/.
func (s *Sandbox) StartDaemon(ctx context.Context, name, description string, command []string, env map[string]string) (DaemonInfo, error) {
	owner := s.daemonOwner
	if owner == "" {
		return DaemonInfo{}, fmt.Errorf("daemons not enabled")
	}
	name = strings.ToLower(strings.TrimSpace(name))
	description = strings.TrimSpace(description)
	if !filenamePattern.MatchString(name) {
		return DaemonInfo{}, fmt.Errorf("invalid name %q: use lowercase [a-z0-9._-]", name)
	}
	if description == "" {
		return DaemonInfo{}, fmt.Errorf("description is required")
	}
	if utf8.RuneCountInString(description) > maxDaemonDescriptionRunes {
		return DaemonInfo{}, fmt.Errorf("description exceeds %d characters", maxDaemonDescriptionRunes)
	}
	if strings.ContainsAny(description, "\n\r") {
		return DaemonInfo{}, fmt.Errorf("description must be a single line")
	}
	script, err := daemonScriptFilename(command)
	if err != nil {
		return DaemonInfo{}, err
	}
	scriptPath := filepath.Join(s.dir, "scripts", script)
	if _, err := os.Stat(scriptPath); err != nil {
		return DaemonInfo{}, fmt.Errorf("script %q not found in sandbox/scripts", script)
	}

	existing, err := s.ListDaemons(ctx)
	if err != nil {
		return DaemonInfo{}, err
	}
	max := s.daemonMax
	if max <= 0 {
		max = defaultDaemonMax
	}
	if len(existing) >= max {
		return DaemonInfo{}, fmt.Errorf("daemon limit reached (%d); stop one before spawning another", max)
	}

	id := randomID()
	shortOwner := strings.ReplaceAll(owner, "-", "")
	if len(shortOwner) > 8 {
		shortOwner = shortOwner[:8]
	}
	containerName := fmt.Sprintf("faultline-daemon-%s-%s", shortOwner, id)
	createdAt := time.Now().UTC().Format(time.RFC3339)
	workHost := filepath.Join(s.dir, "daemons", id)
	if err := os.MkdirAll(workHost, 0o755); err != nil {
		return DaemonInfo{}, fmt.Errorf("create daemon work dir: %w", err)
	}

	args := s.daemonRunArgs(containerName, owner, id, name, description, createdAt, env, command)
	s.logger.Debug("docker run (daemon)", "container", containerName, "args", redactDockerArgs(args))
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return DaemonInfo{}, fmt.Errorf("docker run failed: %w\nOutput: %s", err, string(out))
	}

	return DaemonInfo{
		ID:          id,
		Name:        name,
		Description: description,
		Container:   containerName,
		Status:      "running",
		CreatedAt:   createdAt,
	}, nil
}

// ListDaemons returns all daemon containers owned by this sandbox (running or exited).
func (s *Sandbox) ListDaemons(ctx context.Context) ([]DaemonInfo, error) {
	owner := s.daemonOwner
	if owner == "" {
		return nil, fmt.Errorf("daemons not enabled")
	}
	format := "{{.Names}}\t{{.Status}}\t{{.Label \"" + labelDaemonID + "\"}}\t{{.Label \"" + labelDaemonName + "\"}}\t{{.Label \"" + labelDaemonDesc + "\"}}\t{{.Label \"" + labelDaemonCreatedAt + "\"}}"
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "label="+labelDaemon+"=1",
		"--filter", "label="+labelOwner+"="+owner,
		"--format", format,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w\nOutput: %s", err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	infos := make([]DaemonInfo, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 6 {
			continue
		}
		infos = append(infos, DaemonInfo{
			Container:   parts[0],
			Status:      parts[1],
			ID:          parts[2],
			Name:        parts[3],
			Description: parts[4],
			CreatedAt:   parts[5],
		})
	}
	return infos, nil
}

// FetchDaemonLogs returns the last tail lines of container logs for a daemon id.
func (s *Sandbox) FetchDaemonLogs(ctx context.Context, id string, tail int) (string, error) {
	name, err := s.daemonContainerName(ctx, id)
	if err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = 50
	}
	if tail > 2000 {
		tail = 2000
	}
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", tail), name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs failed: %w\nOutput: %s", err, string(out))
	}
	return string(out), nil
}

// StopDaemon clears restart policy, stops, and removes the container so
// it will not return on host reboot and no longer counts toward the cap.
func (s *Sandbox) StopDaemon(ctx context.Context, id string) (DaemonInfo, error) {
	info, err := s.daemonInfo(ctx, id)
	if err != nil {
		return DaemonInfo{}, err
	}
	name := info.Container
	update := exec.CommandContext(ctx, "docker", "update", "--restart=no", name)
	if out, err := update.CombinedOutput(); err != nil {
		return DaemonInfo{}, fmt.Errorf("docker update failed: %w\nOutput: %s", err, string(out))
	}
	stop := exec.CommandContext(ctx, "docker", "stop", name)
	if out, err := stop.CombinedOutput(); err != nil {
		return DaemonInfo{}, fmt.Errorf("docker stop failed: %w\nOutput: %s", err, string(out))
	}
	rm := exec.CommandContext(ctx, "docker", "rm", name)
	if out, err := rm.CombinedOutput(); err != nil {
		return DaemonInfo{}, fmt.Errorf("docker rm failed: %w\nOutput: %s", err, string(out))
	}
	info.Status = "removed"
	return info, nil
}

func (s *Sandbox) daemonInfo(ctx context.Context, id string) (DaemonInfo, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return DaemonInfo{}, fmt.Errorf("daemon_id is required")
	}
	infos, err := s.ListDaemons(ctx)
	if err != nil {
		return DaemonInfo{}, err
	}
	for _, info := range infos {
		if info.ID == id {
			return info, nil
		}
	}
	return DaemonInfo{}, fmt.Errorf("daemon %q not found", id)
}

func (s *Sandbox) daemonContainerName(ctx context.Context, id string) (string, error) {
	info, err := s.daemonInfo(ctx, id)
	if err != nil {
		return "", err
	}
	return info.Container, nil
}

// daemonScriptFilename requires command to reference /scripts/<flat-file>
// and returns that filename.
func daemonScriptFilename(command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("command is required")
	}
	for _, arg := range command {
		rest, ok := strings.CutPrefix(arg, "/scripts/")
		if !ok {
			continue
		}
		if rest == "" || strings.Contains(rest, "/") || !filenamePattern.MatchString(rest) {
			return "", fmt.Errorf("invalid script path %q: must be /scripts/<flat-filename>", arg)
		}
		return rest, nil
	}
	return "", fmt.Errorf("command must reference a script under /scripts/ (e.g. [\"python3\", \"/scripts/watch.py\"])")
}
