# Faultline Sandbox Image

Multi-runtime execution image for the Faultline agent's sandbox tools.

Published as `ghcr.io/matjam/faultline-sandbox` by the
`.github/workflows/sandbox-image.yml` workflow.

## What's inside

Languages and package managers:

- Python 3 + `pip`
- `uv` + `uvx` (Astral)
- Node.js + `npm` + `npx`
- Bun
- Deno
- Go
- Common browser shared libraries (`libatk`, `libxcomposite`, `libxdamage`,
  GTK, NSS, GBM, etc.) for Playwright/Puppeteer-managed browsers.
  Chromium itself is intentionally not installed by default to keep the image
  smaller.

CLI tools the agent tends to reach for:

- `curl`, `wget`, `git`
- `jq`, `ripgrep`, `fd`, `less`, `tree`, `which`
- `tar`, `gzip`, `unzip`, `zip`, `gnu-netcat`, `make`, `diffutils`, `patch`

Base: `debian:bookworm-slim`. Each image rebuild picks up current Debian
package versions; the *published* image is the reproducible artifact, not
the Dockerfile inputs.

## Contracts the Go side relies on

`internal/adapters/sandbox/docker/` talks to the image via:

- `sh` and `uv` on `PATH`
- Mounts: `/scripts` (ro), `/input` (ro), `/output` (rw), `/venv` (rw),
  `/node` (rw), `/mcp` (rw), `/cache` (rw), `/pyproject.toml`, `/uv.lock`
- Env: `UV_CACHE_DIR=/cache`, `UV_LINK_MODE=copy`,
  `UV_PROJECT_ENVIRONMENT=/venv`, `PATH=/node/node_modules/.bin:...`
- `--user UID:GID` (host user; image must run as any UID)
- `--network=none` by default

Anything else on `PATH` is reachable through the agent's `sandbox_shell`
tool.

## Building locally

```sh
docker build -t faultline-sandbox:dev docker/sandbox
```

Then point `config.toml` at it:

```toml
[sandbox]
enabled = true
image = "faultline-sandbox:dev"
```

## Smoke test

```sh
docker run --rm faultline-sandbox:dev bash -c '
  uv --version
  uvx --version
  python --version
  pip --version
  node --version
  npm --version
  bun --version
  deno --version
  go version
  curl --version | head -1
  jq --version
  rg --version | head -1
  dpkg-query -W libatk-bridge2.0-0 libxcomposite1 libxdamage1 libgtk-3-0 libnss3 libgbm1
'
```

Should print one version line per runtime. If anything is missing, the
build is broken.

## Tags published by CI

- `:latest` — most recent commit on `main`
- `:vX.Y.Z`, `:vX.Y` — release tags (driven by `release-please`)
- `:sha-<short>` — every commit on `main`
- `:pr-<n>` — pull-request preview builds (built but not pushed for
  forks; the workflow only pushes when the head ref is on this repo)

The `config.Default()` shipped with the binary points at `:latest`. Pin
to a versioned tag in your `config.toml` if you want a specific image
version locked down.

## Why Debian

1. **Multi-arch availability.** Debian bookworm publishes both amd64 and
   arm64 images, matching the published sandbox targets.
2. **Browser dependencies are straightforward.** The shared libraries needed
   by browser automation are available from official Debian repositories.
3. **Published image as artifact.** Package versions are intentionally not
   pinned individually; the pushed image tag is the reproducible unit.
