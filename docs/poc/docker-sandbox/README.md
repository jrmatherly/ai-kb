# Docker sandbox backend feasibility PoC

For the current implementation and deployment requirements, see [Sandbox deployment and troubleshooting](../../../website-docs/06-development/04-sandbox-deployment.md). This PoC is kept only as a historical feasibility experiment and does not describe the current adapter's behavior.

The program calls the Docker Engine API directly and checks, item by item, whether Docker can carry the AI KB
`RemoteSandboxClient` contract plus the E2B snapshot workflow. It is not product code and is not part of the main module build
(it has its own `go.mod`). It exists only as reproducible evidence for the research findings.

Every item prints `PASS/FAIL` and the observed value. Steps marked `(GAP)` assert what Docker cannot do,
and they also end in PASS: PASS means the gap was reproduced, not that the capability exists.

## Running it

You need a reachable Docker daemon (`DOCKER_HOST` is respected) that can pull `python:3.11-slim`:

```bash
cd docs/poc/docker-sandbox
go run .            # may need sudo -E when the daemon is on a local unix socket
```

The program builds its own template image with a uid 1000 `user` account (matching the E2B template convention) and deletes the containers it created when it finishes.
Remove the `ai-kb-poc/*` images it leaves behind with `docker image rm`.

## What it covers

- Lifecycle: Create / Connect (reconnect with a new client) / List (label filter) / Delete / Pause / Stop+Start
- Exec: user, workdir, env, stdin, separate stdout and stderr, exit code, timeout
- Files: read, write and stat through the archive API; mkdir/ls/rm through exec
- Session semantics: `pip install` and written files survive across execs
- Snapshots: commit to an image, start a new sandbox from a snapshot, v1 to v2 increments, list by label, layer count and size
- Gaps reproduced: a client cancel does not kill the process, root cannot get past permission bits after CapDrop ALL, snapshots do not include memory state,
  the daemon has no idle TTL, killing `docker run` leaves the container running
