# Sandbox deployment and troubleshooting

How skills are used is covered in [Skills and sandbox](../03-features/22-skills-sandbox.md), and the field definitions in [Sandbox and skills API](../04-api/02-api-sandbox-skills.md). This page adds the operational requirements for deployment, templates, the desktop and backend integration.

## Backends and templates

A workspace's named sandbox configs can use Docker, CubeSandbox or E2B, and this page covers deploying those three backends. The Lite desktop client also has a `host` backend that relies on operating-system isolation. It is not a server-side named config; its build requirements are in [Lite host sandbox](#lite-host-sandbox) below. The old `local` backend, which ran bare host processes, has been removed. Docker talks to the Engine API directly. E2B uses the control-plane REST API and the envd data plane. Cube keeps a dedicated adapter for templates and network policy.

The standard image is defined by `docker/Dockerfile.sandbox` and contains Python 3.12, Node.js 20, Bash, jq and `/workspace`. Commands and file operations run as root inside the sandbox by default. A `user` account with UID 1000 is kept for explicit per-account execution. Isolation between sessions comes from the container or remote sandbox. The working-directory convention is not a file-permission limit on root.

| Build target | Image tag suffix | Purpose |
| --- | --- | --- |
| `sandbox` | none | Docker session image; base image for the E2B CLI template |
| `cube` | `-cube` | Cube CLI template, includes envd |
| `desktop` | `-desktop` | Base image for the E2B graphical desktop template |
| `desktop-cube` | `-desktop-cube` | Cube graphical desktop template, includes envd |

The image name is `ghcr.io/jrmatherly/ai-kb-sandbox:<version><suffix>`. A template ID belongs to one cluster or account, so do not copy another deployment's IDs. A production template should match an application version you have verified. The repository Dockerfile and release workflow are authoritative for build targets and published architectures.

## Docker deployment

The Docker backend is off by default. A system administrator turns it on under Settings → System Settings → Network security. Before that setting is stored, `AIKB_SANDBOX_DOCKER_ENABLED=true` works as a fallback. When it is off, existing configs can still be viewed and deleted, but no new session containers are created.

One config connects to one daemon, and one session uses one long-lived container. There is no cross-host scheduling. If app replicas each connect to their own local daemon, do not assume they share containers or skill snapshots.

| Setting | Default or requirement |
| --- | --- |
| Daemon address | When empty, `DOCKER_HOST` or the current Docker context is detected; a remote daemon uses `tcp://host:2376` |
| TLS certificate directory | Required for remote TCP: a directory readable by the app containing `ca.pem`, `cert.pem` and `key.pem` |
| CPU / memory / PIDs | Default 2 cores / 2 GiB / 512 processes |
| Idle TTL | Default 1800 seconds |
| Network mode | Only `bridge` or `none`; `host`, `container:` and custom network names are rejected |
| OCI runtime | Optional, any runtime installed on the daemon, such as `runsc` |

When the app runs in a container, mount the real Docker socket or connect to a remote daemon instead. The socket grants control of the host's Docker, so give it only to a trusted app. Before dropping privileges, the entrypoint script adds appuser to the group matching the socket's GID. Do not open the socket with `chmod 666`. If the socket is `root:root` and writable only by its owner, first give it a suitable non-root group on the host.

Prefer the standard image. A custom image must meet the adapter's requirements: root, Bash, GNU `find -printf`, coreutils `timeout` and a writable workspace. Session file checkpoints and rewind also need Git. Cancelling an HTTP request does not stop the process in the container, so the adapter enforces command timeouts with `timeout` inside the container.

The idle sweep is triggered by Create and Connect and runs in the background. It judges idleness by the TTL recorded when each container was created; it is not a timer built into the daemon. There is currently no hard lifetime limit, and a script that can run commands can keep refreshing the activity marker, so monitor long-lived containers separately.

### Skill snapshots and disk

Installing a skill runs `docker commit` to produce a local image under `weknora-skill/`, and later sessions start from that install snapshot. It saves the filesystem, not memory state. Workspace-shared volumes and the Docker graphical desktop are not supported yet.

An incremental snapshot inherits the old image's layers, so deleting an old tag does not necessarily free disk. Uninstalling a skill adds a layer of delete markers, and the original files can remain in parent layers. The current flow does not flatten images automatically or distribute them across daemons. Monitor layer count and disk usage, and confirm session and snapshot references before cleaning up. When you need a fresh base image, create a new config and reinstall the skills.

## CubeSandbox / E2B setup

1. Prepare a working control plane, data-plane gateway and sandbox domain. In the workspace settings, fill in the API, Proxy, domain and credentials. Private or loopback endpoints need the private-address option turned on explicitly; cloud metadata addresses stay blocked.
2. Click "Connect and continue". It checks the control plane and then loads the template catalog. A successful connection only proves the control plane works.
3. If the standard template is missing, create it explicitly; if you need a desktop, create a desktop template as well. Wait until the status is `READY` before selecting it. Cube should use the `-cube` variant that includes envd; a plain Docker image does not serve `:49983/health`.
4. Run "Full verification". It creates a real sandbox, runs a probe and destroys it, which confirms the data plane and script environment also work.
5. After saving, select the config on the agent and check attachment reading, command state persistence and artifact download.

`proxy_url` is for a self-hosted data-plane gateway: it keeps the sandbox Host routing when connecting to the gateway, which suits environments without wildcard DNS. When the control plane is reachable but execution fails, check the Proxy, sandbox domain, inbound credentials and whether the app can reach the gateway.

Cube guest DNS belongs to the template configuration. After changing DNS or the image, rebuild the template before the new environment takes effect. A config that already has skills installed cannot change or rebuild its base image, and cannot switch between CLI and desktop. Create a new config and install the skills there, so snapshots never diverge from their base image.

Multiple replicas must share a Redis instance, which stores the session→sandbox binding and desktop state. Only single-instance development can use the in-memory store. Existing sandboxes do not pick up backend config changes automatically; the `next_turn` / `new_session` policies for skill updates are in [Skills and sandbox](../03-features/22-skills-sandbox.md).

| Symptom | What to check |
| --- | --- |
| Connection check fails | Whether the API field holds the Dashboard address by mistake; credentials, TLS and the private-address option |
| Connects but execution fails | Proxy, sandbox domain, gateway routing and inbound token |
| Template build fails | The build error returned by the cluster; whether the image can be pulled, the architecture matches, and the Cube image includes envd |
| Sandbox has no outbound network | Template network policy, guest DNS, cluster egress proxy; allowing a private control plane does not let scripts reach the network |
| Dependency install fails | Whether package sources are allowed when outbound traffic is denied by default; skill install and sessions use the same network policy |
| State lost after reconnect | Whether Redis is shared, whether the TTL expired, whether a skill update triggered a rebuild |

## Interactive terminal and graphical desktop

The terminal and desktop in the conversation sidebar both connect to the session sandbox over WebSocket. The terminal supports Cube, E2B and Docker; the desktop supports only Cube and E2B. A browser cannot send an authentication header in the WebSocket handshake, so both first exchange a logged-in POST for a short-lived ticket valid for two minutes, then put the ticket in the handshake query:

| Capability | Get a ticket | WebSocket |
| --- | --- | --- |
| Terminal | `POST /api/v1/sessions/:session_id/sandbox/terminal-ticket` | `GET /api/v1/sessions/:id/sandbox/terminal?ticket=...` |
| Desktop | `POST /api/v1/sessions/:session_id/sandbox/desktop-ticket` | `GET /api/v1/sessions/:id/sandbox/desktop?ticket=...` |

The ingress proxy must forward the WebSocket Upgrade for both paths, relax its read timeout, and keep the ticket query out of its access log. The standard frontend Nginx already configures a query-free log format for `^/api/v1/sessions/[^/]+/sandbox/(terminal|desktop)$`; a custom Ingress must do the same. Once the terminal is connected, the server re-checks login state, workspace membership and session ownership about once a minute; logging out or being removed from the workspace disconnects the terminal. Parameters are in the [terminal API](../04-api/02-api-sandbox-skills.md#session-terminal) and the [desktop API](../04-api/02-api-chat.md#sandbox-desktop).

The terminal and desktop disconnect after the configured `terminal_idle_disconnect_sec` with no activity (default 900 seconds, minimum 60 seconds, maximum 24 hours), after which the sandbox pauses on the provider's TTL. The terminal counts keyboard input and PTY output as activity; the desktop counts keyboard, mouse and clipboard input.

The Docker terminal is an exec with `TTY=true`, and the Engine API cannot reattach to it, so it behaves differently from Cube and E2B:

- While the terminal is open, the activity marker used for idle reclamation is refreshed periodically, so `DockerIdleTTL` does not reclaim the container. After the terminal closes, `DockerIdleTTL` applies as usual.
- When the terminal disconnects (panel closed, idle disconnect, network loss), AI KB sends SIGHUP to the shell and its child processes, as closing a terminal window would. Foreground jobs end; background jobs started with `nohup` keep running.
- The frontend does not reconnect a Docker terminal automatically. Clicking "Reconnect" opens a new shell.
- When the sandbox is not running (the container was stopped or reclaimed after being idle), the panel says it may have been reclaimed. If the container was deleted, starting the terminal creates a new sandbox and earlier files are not kept.

### Graphical desktop

Only Cube and E2B desktop templates support the sidebar desktop. The backend starts the desktop processes on first open. The config must select a desktop template and set `desktop_enabled`. Once skills are installed, the base image cannot be switched in place.

```text
Browser noVNC → AI KB ticket relay → provider gateway → websockify :6080 → local x11vnc :5900
```

The browser never holds the sandbox API key, the inbound token or the websockify password. A desktop ticket can be consumed only once; the full definition is in the [chat API](../04-api/02-api-chat.md#sandbox-desktop). The Cube desktop template's `exposedPorts` exposes only envd's 49983. **Do not add 6080 to the host NAT**: the desktop must go through the gateway and the AI KB relay.

Each session allows only one desktop relay at a time, and Redis coordinates the slot across replicas. After a sandbox rebuild the relay disconnects with `SANDBOX_REBUILT`. Do not treat the new desktop as the old instance with its temporary files intact. Idleness is judged by RFB keyboard, mouse and clipboard messages; framebuffer refresh requests do not count as user activity.

## Lite host sandbox

The `host` backend is compiled only into the Lite desktop app built with the `desktop` build tag (`build:tags` in `cmd/desktop/wails.json`). The server and the single-binary Lite do not include it. It runs commands for sessions that have no sandbox config selected; usage is in [Skills and sandbox](../03-features/22-skills-sandbox.md#lite-host).

| Platform | Status |
| --- | --- |
| macOS | Runs each command under the system `sandbox-exec` (Seatbelt); if it is unavailable at startup, the backend is not enabled |
| Windows | Not implemented yet; reports unavailable and never falls back to unisolated execution |
| Linux | Not supported |

Each command is a new local process with no session-level instance, so nothing is written to the session's sandbox binding and no workspace checkpoints are taken. Project directories the user approves through the system directory picker are stored in `project_dirs` in `desktop-prefs.json`. A session can bind only a directory on that list exactly, not a subdirectory of one or a typed path. `approval_mode` currently supports only `auto` (free reads and writes inside the workspace, with outbound network access); writing any other value is rejected. The preferences file location is on [Desktop client](../05-clients/05-desktop.md#preference-storage-cmd-desktop-prefs-go).

## Development verification

Unit tests do not need a real daemon:

```bash
go test ./internal/sandbox -run 'TestDocker' -count=1
```

Real Docker verification needs the standard image built first, and it creates test containers:

```bash
docker build -f docker/Dockerfile.sandbox --target sandbox -t ghcr.io/jrmatherly/ai-kb-sandbox:dev .
DOCKER_INTEGRATION_IMAGE=ghcr.io/jrmatherly/ai-kb-sandbox:dev \
go test -tags=docker_integration ./internal/sandbox \
  -run '^TestDocker.*Integration' -count=1 -v -timeout=15m
```

The conformance tests for remote backends are `internal/sandbox/cube_integration_test.go` and `e2b_compatible_integration_test.go`; configure the test cluster credentials as described at the top of each file before running them. Cover session state persistence, shell reuse, attachment staging, artifact collection and timeouts, not just Health. After the tests, confirm the test instances were cleaned up.
