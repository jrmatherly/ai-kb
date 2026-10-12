# Skills and sandboxes

A skill is a `SKILL.md` description plus its scripts, templates and reference material. A sandbox is the environment that runs those scripts. Add a skill to the workspace catalog, install it into a sandbox, and select that sandbox and skill in an agent; the skill is then available in Smart Reasoning conversations.

## Configure and use a skill

To run a skill for the first time you create a sandbox configuration, install the skill into it, bind both to an agent and ask a question.

1. A workspace Admin or Owner creates a configuration under Settings → Sandbox Config, picks Docker, CubeSandbox or E2B, and fills in the connection details.
2. The wizard connects to the cluster and lists templates. Run "Full verification" when the whole chain needs checking: it creates a real sandbox, runs a probe script, checks outbound access and destroys the sandbox.
3. In the sidebar, Toolbox → Skill Management adds a ZIP or a source link, then selects the sandboxes to install into (the old Settings → Skill Management link redirects here). A catalog entry only means the package was saved; the skill runs only once its install status is ready.
4. Open the "Skills" section of the agent editor, select the sandbox, and set the skill scope to all, selected or none. Skill execution is a Smart Reasoning mode feature.
5. Ask a question in a conversation, or write `@skill` to steer the agent toward that skill. Upload attachments when the skill needs files; generated deliverables appear in the "Files" tab of the "Sandbox" panel on the right of the conversation, where they can be previewed and downloaded.

`@skill` does not revoke the agent's access to its other authorized skills. The execution tools are not registered when no sandbox is selected (the Lite desktop app on macOS switches to the [host sandbox](#lite-host) instead), when the workspace has disabled scripts, or when the required backend capability is unavailable; no prompt can bypass these conditions.

<Screenshot
  src="/screenshots/skill-catalog.png"
  caption="Workspace skill catalog: skills and their install status per sandbox" />

## Manage installs and updates

The skill management page is where you add, install, upgrade, disable and remove skills, and where you watch an install run.

The page supports search by name and description. Search is case-insensitive and every space-separated keyword must match. When nothing matches, clear the search or add the skill.

The workspace catalog holds one copy of each skill package; each sandbox configuration keeps its own install record and an image snapshot that contains the skill. The same skill can be installed into several sandboxes, each with its own progress and failure reason.

| Action | Result |
| --- | --- |
| Add to catalog | Saves the package, name, version and description; does not install |
| Install | Builds the runtime environment in the selected sandbox, installs dependencies, verifies the skill loads, then publishes a new snapshot |
| Retry | Installs again from the package already saved for that sandbox, optionally with install notes; a package that is already ready can be skipped |
| Upgrade | After the catalog registers a new version, older installs are marked upgradable; upgrading installs the catalog version into the selected sandbox |
| Stop install | Aborts an install in progress; the status becomes failed and the skill can then be retried or uninstalled |
| Disable | Keeps the install record but stops agents from selecting the skill |
| Uninstall from sandbox | Updates that sandbox's skill image and keeps the package in the catalog so other sandboxes can still install it |
| Delete catalog entry | Requires that no sandbox install references it; it never uninstalls from sandboxes implicitly |

The install page shows a percentage, the current stage and the log; the full run is kept in the install record. Closing the progress drawer or losing the progress stream does not stop the install. When there is no live progress, refresh the skill status; an expired event log does not mean the package is lost.

Installs are run by the built-in installer agent. It reads `SKILL.md`, installs the dependencies and checks the prerequisites the skill needs, such as external command-line tools. When a required command is missing or a prerequisite is unresolved (for example a service that must be deployed separately), the install fails with the reason; a skill is never marked ready merely because it has no dependency list. While an install is running, an administrator can add notes in the install progress drawer, such as a CLI to install, installation docs or environment limits; after the install ends, a reinstall can carry notes as well.

When an upgrade or retry fails, the sandbox keeps serving the previous ready version and agents can still use it; the new version replaces it only after a successful install.

`skill_rollout` controls image rollout: the default `next_turn` rebuilds the sandbox of an existing session on its next turn; `new_session` makes only sessions created afterwards use the new image. A rebuild discards the temporary runtime state of the old instance, so files to deliver belong in `/workspace/output`, where the system collects them.

### Supported sources

| Input | Description |
| --- | --- |
| ZIP file | Export the skill directory and upload it |
| `@owner/slug`, `@owner/slug@1.2.0` | A ClawHub skill by author and version |
| `slug`, `slug@1.2.0` | A ClawHub slug |
| ClawHub, SkillHub and self-hosted SkillHub pages | Resolved through the matching source |
| GitHub or GitLab repository or directory URL | Fetches the matching skill package |
| `https://skills.sh/owner/repo/slug`, a ClawHub skills.sh page | Resolved by the install resolver to a specific version and directory in the repository |
| Direct ZIP or SKILL.md URL | Downloads the package or the entry file |

Sources must be readable anonymously; downloads never carry the user's private repository credentials. Export a private skill as a ZIP first. `owner/slug` is ambiguous; use `@owner/slug` or the full URL.

The skill package limit is separate from the ordinary document limit: `MAX_SKILL_BUNDLE_SIZE_MB` defaults to 256 MiB, is never lower than `MAX_FILE_SIZE_MB` when unset, and caps at 512 MiB. A GitHub download is measured as the whole repository archive, not just the skill subdirectory. After changing it, restart app and frontend so the application and Nginx limits agree.

## Choose a sandbox backend

To pick a backend you weigh what each one needs from you against how it runs sessions.

| Backend | Required fields | How it runs |
| --- | --- | --- |
| Docker | Image; optional daemon address, TLS certificate directory, CPU, memory and PID limits, network mode, runtime, idle TTL | One long-lived container per session |
| CubeSandbox | Control plane address, data plane proxy, sandbox domain, template; an API key per cluster configuration | A session-scoped remote sandbox |
| E2B | API key, template; for self-hosting also the API address, sandbox domain and data plane proxy | E2B Cloud or an E2B-compatible control plane |
| host | Nothing to fill in. Lite desktop app only, macOS only for now | When the agent has no sandbox configuration selected, runs inside the local project directory; the operating system blocks anything outside it |

The `local` host-process backend was removed: it ran bare on the host with no isolation. Lite's `host` is not one of the named workspace configurations above and is not a replacement for `local`. The full field list of the current configuration is in the [Sandbox and skills API](../04-api/02-api-sandbox-skills.md).

The Docker backend is off by default. A system administrator enables it under System Settings → Network Security, or sets `AIKB_SANDBOX_DOCKER_ENABLED=true` as the fallback while no setting row exists. A local connection also needs the real Docker socket mounted into app, which gives app control of the host's Docker. A remote TCP daemon needs a TLS certificate directory containing `ca.pem`, `cert.pem` and `key.pem`. The Docker network accepts only `bridge` or `none`, and an installed OCI runtime such as `runsc` can be selected.

For self-hosted E2B or Cube, `proxy_url` points at the data plane gateway: AI KB connects to the gateway but keeps the sandbox Host header, for clusters without wildcard DNS. `allow_private_endpoints` permits private-network and loopback cluster addresses; link-local and cloud metadata addresses stay blocked. It is a separate setting from whether scripts inside the sandbox can reach the network.

Scripts run as the sandbox's `root` account by default; the template's `user` account must be selected explicitly. Isolation comes from the container or the remote sandbox. `/workspace` is only the agreed working directory and does not limit what root commands can read or write.

### Network policy

`config.network` on Cube and E2B applies to conversation sandboxes, skill installs and full verification alike:

- Egress is allowed by default; `deny_egress_by_default=true` switches to deny by default, after which `allow_out` admits IPs, CIDRs or domains.
- `deny_out` accepts IPv4 addresses and CIDRs; domain allow rules require deny by default.
- Cube's `cube_rules` set rules by Host or SNI, method and path, with ordering, audit level and HTTPS header injection; E2B's `e2b_host_rules` configure request header injection for domains already allowed.
- Injected credentials are stored encrypted and masked in responses. Inbound always requires credentials; the legacy `allow_public_inbound` field does not open anonymous inbound access.
- Docker controls egress with `docker.network_mode` and cannot take the fine-grained Cube and E2B rules. With egress denied by default, the package sources that dependency installs need must be allowed explicitly too.

Existing instances do not pick up a changed policy automatically; verify after creating or rebuilding a sandbox. Before a backend identity change or a configuration delete, the system queries running and paused instances and the agents bound to the configuration; when any are in use, the operation is refused and the settings page shows what is using it.

### Lite host sandbox (host) {#lite-host}

Since v0.8.2 the [Lite desktop app](../05-clients/05-desktop.md) on macOS uses the system Seatbelt sandbox to isolate the agent's command and file tools. An agent with a Docker, E2B or Cube configuration selected still uses that remote sandbox; the host sandbox is used only when no sandbox configuration is selected. Windows and Linux do not provide this capability yet, so there an agent without a configuration cannot run commands.

- **Working directory**: on the new conversation page, click "Select project" and choose the project folder in the system directory picker; the conversation title bar shows the project name. Without a selection the title shows "Temporary workspace" and a session directory is created under `~/Documents/WeKnoraLite/<date>/session-*`. The home directory itself, a directory that contains it, and directories under `~/Library` cannot be selected.
- **Paths**: the workspace is a real local path; there is no `/workspace`, `/workspace/input` or `/workspace/output`. The agent edits files in the project directly, those files do not enter the conversation's artifact list, and deleting the conversation does not delete the local directory.
- **Limits**: in the default `auto` mode commands keep outbound network access (only `ask` mode denies it), can write only inside the project directory, and the project's `.git` is read-only. Apart from common development toolchains (such as `.nvm`, `.pyenv` and `.cargo`) the home directory is unreadable, and credential locations such as `.ssh`, `.aws` and the keychain are never readable.
- **Rollback**: rolling back a conversation does not restore local project files; manage them with Git when you need that.

## Environment variables and credentials

To give a skill a credential you set it at the layer that matches who should see it: the workspace, a skill, or only yourself.

Personal variables live under Settings → Sandbox secrets; workspace-level skill variables are set on the skill card under Toolbox → Skill Management. Lists show the declaration, whether it is set and where it comes from; secret values are never echoed back.

| Layer | Purpose |
| --- | --- |
| Workspace sandbox `config.env_vars` | Injected into every sandbox created from that configuration, for its scripts |
| Workspace skill variables | Default values an administrator fills in for the variables a skill declared |
| Personal sandbox variables | Used when you run commands in that sandbox configuration |
| Personal skill variables | Used when you run that skill |

Skill variables resolve as **personal skill value > personal sandbox value > workspace skill value**; a lower layer is used only for names that are unset above it. The workspace sandbox environment is part of the runtime and should not hold secrets that scripts must not read. Deleting a personal override falls back to the lower layer; disabling a skill does not delete personal credentials.

Personal skill variables accept only names the skill declared, which may include `AIKB_*` credentials the skill needs. Personal sandbox variables refuse the names reserved by the sandbox: `PATH`, `HOME`, `USER`, `SHELL`, `LD_PRELOAD`, `LD_LIBRARY_PATH`, `PYTHONPATH`, `PYTHONHOME`, `NODE_OPTIONS`, and any `AIKB_SKILL_*` name. Variables the system detects in executed commands only fill in unset personal values; they never overwrite an existing personal or workspace setting. Fields and examples are in the [personal variable API](../04-api/02-api-sandbox-skills.md#personal-environment-variables).

## Generate and download files

To produce a deliverable the agent reads the skill, runs its scripts, writes the result under `/workspace/output` and the conversation offers it for download.

`read_file(path="skill://<name>/SKILL.md")` reads the description; bundled scripts run through `shell_exec(skill_name=..., command=...)`. In a command, `$AIKB_SKILL_DIR` points at the directory the skill is installed in; `skill://` is a read address and cannot be used as a shell path.

Attachments are staged in `/workspace/input`, working scripts go in `/workspace`, and downloadable artifacts go in `/workspace/output`. Use `write_sandbox_file` to create or append, `edit_sandbox_file` for in-place replacement, and `read_file` to read in pages. File tool boundaries, output budgets and rebuild behavior are in [Agent engine](07-agent.md); the delivery entry points are in [Sessions and the chat experience](18-chat-experience.md).

<Screenshot
  src="/screenshots/skill-sandbox-chat.png"
  caption="A Word file generated in the sandbox, previewed and downloaded in the conversation" />

While a command runs, the tool card in the conversation shows the last few lines of its output live, which helps judge the progress of a long command; this intermediate output is not written into the model context.

### Sandbox panel

In a conversation that uses a remote sandbox, the "Sandbox" panel on the right has three tabs:

| Tab | Content | Available on |
| --- | --- | --- |
| Files | Downloadable files generated this turn or across all turns, searchable by file name | Every remote sandbox |
| Terminal | An interactive shell (xterm) connected to this session's sandbox. On Cube and E2B a page refresh reattaches to a shell that is still running; on Docker a disconnect ends the shell (background jobs started with nohup keep running) and reconnecting opens a new one | Cube, E2B, Docker |
| Desktop | The XFCE graphical desktop inside the sandbox, driven from the browser; one connection per session at a time | Cube and E2B desktop templates, with `desktop_enabled` on in the configuration |

Opening the panel does not create or wake a sandbox. When the sandbox is running, the terminal and desktop connect directly; when it is paused or not yet created, click "Start terminal", "Connect desktop" or "Create and start", and the woken or new sandbox is billed per the workspace configuration. A terminal or desktop left idle disconnects automatically, after which the sandbox pauses on the provider's TTL. The idle time is the configuration's "Terminal / desktop idle disconnect (s)", default 900 seconds, range 60 seconds to 24 hours. When a skill update triggers a sandbox rebuild, unsaved work in the previous terminal and desktop is lost.

<Screenshot
  src="/screenshots/sandbox-panel-terminal.png"
  caption="Sandbox panel: files, terminal and desktop beside the conversation"
  hint="A conversation using a Cube or E2B sandbox, with the 'Sandbox' panel on the right switched to the 'Terminal' tab: a colored user@host prompt and one command's output in the terminal, and the three tabs 'Files / Terminal / Desktop' visible at the top of the panel." />

## Deployment and troubleshooting

Docker socket and TLS, template versions, remote gateways, multi-replica Redis, skill snapshot disk usage, terminal and desktop relays, and the build requirements of the Lite host sandbox are covered in [Sandbox deployment and troubleshooting](../06-development/04-sandbox-deployment.md).

## Implementation reference

- `internal/handler/sandbox_config.go`, `sandbox_skill.go`, `skill_catalog.go`, `me_env_var.go`
- `internal/application/service/tenant_skill_install.go`, `tenant_skill_runtime_verify.go`, `tenant_skill_steer.go`, `user_env_resolver.go`
- `internal/types/tenant.go`, `sandbox_network_policy.go`
- `internal/sandbox/remote_client.go`, `docker_remote_client.go`, `gateway_transport.go`, `terminal.go`
- `internal/localsandbox/` (Lite host sandbox: `core/policy_builder.go`, `seatbelt/`, `adapter/`)
