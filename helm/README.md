# AI KB Helm Chart

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Helm chart for deploying [AI KB](https://github.com/jrmatherly/ai-kb) - an AI-powered Knowledge RAG Platform.

## Overview

AI KB is an intelligent knowledge base platform that combines:

- Document parsing and understanding
- Vector search with BM25 hybrid retrieval
- LLM integration for conversational AI
- Multi-tenant support with encryption

## Prerequisites

- Kubernetes 1.25+
- Helm 3.10+
- PV provisioner support in the underlying infrastructure
- Ingress controller (nginx-ingress recommended) for external access

## Quick Start

The chart is published to GHCR on every chart version bump:

```bash
helm install ai-kb oci://ghcr.io/jrmatherly/ai-kb/charts/ai-kb --version 0.3.0 \
  --set secrets.dbPassword=<secure-password> \
  --set secrets.redisPassword=<secure-password> \
  --set secrets.jwtSecret=<secure-random-string>
```

Flux users reference the same path from an `OCIRepository` with `ref.tag` set to the chart
version. `make bump-version` also bumps the chart patch version, so each product release
publishes a chart that pins its image tag. The commands below install from a checkout instead.

The chart pulls `ghcr.io/jrmatherly/ai-kb-{app,ui,website,docreader}` at the tag in
`Chart.appVersion`. To use your own registry, build the images (`make build-images`),
push them and point `app.image`, `frontend.image`, `website.image` and `docreader.image`
at it.

```bash
# Add required secrets
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  --set secrets.dbPassword=<your-db-password> \
  --set secrets.redisPassword=<your-redis-password> \
  --set secrets.jwtSecret=<your-jwt-secret>
```

## Architecture

```text
                    ┌─────────────┐
                    │   Ingress   │
                    └──────┬──────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
           ▼                               ▼
    ┌─────────────┐                 ┌─────────────┐
    │  Frontend   │                 │   Backend   │
    │  (Vue.js)   │                 │   (Go/Gin)  │
    │  + Website  │                 └──────┬──────┘
    │  (/docs)    │                        │
    └─────────────┘                        │
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
                    ▼                      ▼                      ▼
             ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
             │  Docreader  │        │  PostgreSQL │        │    Redis    │
             │   (gRPC)    │        │  (ParadeDB) │        │   (Queue)   │
             └─────────────┘        └─────────────┘        └─────────────┘
```

## Installation

### Basic Installation

```bash
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### With Ingress

```bash
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  --set ingress.enabled=true \
  --set ingress.host=ai-kb.example.com \
  --set ingress.tls.enabled=true \
  --set ingress.tls.secretName=ai-kb-tls \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### With External LLM (Ollama)

```bash
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  --set app.extraEnv[0].name=OLLAMA_BASE_URL \
  --set app.extraEnv[0].value=http://ollama.ollama:11434 \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### Production Installation

For production with the default local file storage, use a single App replica and
an update strategy that releases the old Pod's volume before starting its
replacement:

```yaml
# values-production.yaml
global:
  storageClass: "fast-ssd"

app:
  replicaCount: 1
  strategy:
    type: Recreate
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2
      memory: 4Gi

postgresql:
  persistence:
    size: 100Gi

ingress:
  enabled: true
  host: aikb.company.com
  tls:
    enabled: true
    secretName: ai-kb-tls

secrets:
  existingSecret: ai-kb-secrets  # Use pre-created secret
```

```bash
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  -f values-production.yaml
```

The chart-created data-files PVC uses `ReadWriteOnce` (RWO). RWO allows several
Pods to mount the volume on the same node; it does not provide shared writable
access across nodes. With a single-node-attach volume, the default rolling update
(`maxSurge: 1`, `maxUnavailable: 0`) can leave a replacement Pod waiting for the
old Pod's attachment to be released. `Recreate` terminates the old Pods before
starting replacements during an upgrade, so **App upgrades have downtime**;
volume detach/attach can still take time. This strategy does not make RWO storage
suitable for multiple App replicas across nodes. See Kubernetes documentation on
[access modes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes)
and [Deployment strategies](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy).

For multiple App replicas using local storage, first provision and verify a
shared filesystem PVC that supports `ReadWriteMany` (RWX) on the intended nodes.
Apply the following override together with `values-production.yaml`:

```yaml
# values-shared-storage.yaml
app:
  replicaCount: 3
  strategy:
    type: RollingUpdate

dataFiles:
  persistence:
    enabled: true
    existingClaim: ai-kb-shared-files  # Pre-provisioned, verified RWX PVC
```

```bash
helm install ai-kb ./helm \
  --namespace ai-kb \
  --create-namespace \
  -f values-production.yaml \
  -f values-shared-storage.yaml
```

The existing claim must be in the release namespace and allow the App containers
to read and write `/data/files`. The chart references it without creating or
validating its access mode. All App replicas must see the same files; disabling
persistence or assigning independent local volumes does not provide shared
storage. This override addresses App file storage only, not high availability
for the chart's database or other components. Object storage is another option
only after configuring the provider and checking that all paths needed by your
deployment are accessible to every replica.

## Configuration

### Service names

Optional Service name overrides preserve the following defaults:

| Parameter | Default when omitted or empty |
| ----------- | ------------------------------- |
| `app.service.name` | `app` |
| `frontend.service.name` | `frontend` |
| `website.service.name` | `website` |
| `postgresql.service.name` | `postgres` |
| `redis.service.name` | `redis` |
| `docreader.service.name` | `docreader` |
| `neo4j.service.name` | `neo4j` (when enabled) |

For example, save this as `service-names.yaml`:

```yaml
app:        {service: {name: ai-kb-app}}
frontend:   {service: {name: ai-kb-frontend}}
website:    {service: {name: ai-kb-website}}
postgresql: {service: {name: ai-kb-postgres}}
redis:      {service: {name: ai-kb-redis}}
docreader:  {service: {name: ai-kb-docreader}}
neo4j:      {service: {name: ai-kb-neo4j}}
```

Pass `-f service-names.yaml` alongside your other values. Names must be valid
Kubernetes Service names. Services, backend addresses, Ingress and installation
notes use these names consistently. Frontend's `APP_HOST` follows the App Service
unless `frontend.appHost` is explicitly set.

For multiple releases in one namespace, use distinct names for every enabled
Service (for example, `alpha-app` and `beta-app`). Defaults are not release-prefixed
and still collide. Configure separate Ingress hosts when needed.

Omitted or empty overrides preserve Service names on upgrade. Changing a name on
an existing installation requires a DNS/consumer cutover plan; matching a
pre-existing Service name does not automatically transfer its ownership to Helm.

### Global Parameters

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `global.storageClass` | Storage class for PVCs | `""` |
| `global.imagePullSecrets` | Image pull secrets | `[]` |
| `global.maxFileSizeMB` | Maximum upload size in MB (frontend, app, docreader) | `50` |
| `global.podSecurityContext` | Default pod security context (`runAsNonRoot`, seccomp) | See values.yaml |
| `<component>.podSecurityContext` | Per-component pod override (`fsGroup`) | See values.yaml |
| `<component>.securityContext` | Per-component container context (user, read-only root, dropped capabilities) | See values.yaml |

### ServiceAccount

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |

### App (Backend)

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `app.enabled` | Enable backend | `true` |
| `app.replicaCount` | Number of replicas | `1` |
| `app.strategy.type` | App update strategy: `RollingUpdate` or `Recreate` | `RollingUpdate` |
| `app.strategy.rollingUpdate.maxSurge` | Extra Pods during rolling updates (integer or percentage); ignored with `Recreate` | `1` |
| `app.strategy.rollingUpdate.maxUnavailable` | Unavailable Pods during rolling updates (integer or percentage); ignored with `Recreate` | `0` |
| `app.image.repository` | Image repository | `ghcr.io/jrmatherly/ai-kb-app` |
| `app.image.tag` | Image tag | `""` (uses appVersion) |
| `app.resources` | Resource limits | See values.yaml |
| `app.env` | Environment variables | See values.yaml |
| `app.extraEnv` | Additional env vars | `[]` |

### Frontend

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `frontend.enabled` | Enable frontend | `true` |
| `frontend.replicaCount` | Number of replicas | `1` |
| `frontend.image.repository` | Image repository | `ghcr.io/jrmatherly/ai-kb-ui` |
| `frontend.image.tag` | Image tag | `""` (uses appVersion) |
| `frontend.appHost` | Backend host the frontend proxies to | App Service name |
| `frontend.appPort` | Backend port the frontend proxies to | `app.service.port` |

### Website

The documentation image. The Ingress forwards `/docs` to it and everything else to the
frontend, so the docs share the application's host (`https://<ingress.host>/docs/`).

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `website.enabled` | Enable the website | `true` |
| `website.replicaCount` | Number of replicas | `1` |
| `website.image.repository` | Image repository | `ghcr.io/jrmatherly/ai-kb-website` |
| `website.image.tag` | Image tag | `""` (uses appVersion) |

### DocReader

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `docreader.enabled` | Enable DocReader | `true` |
| `docreader.replicaCount` | Number of replicas | `1` |
| `docreader.image.repository` | Image repository | `ghcr.io/jrmatherly/ai-kb-docreader` |
| `docreader.image.tag` | Image tag | `""` (uses appVersion) |

### PostgreSQL (ParadeDB)

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `postgresql.enabled` | Enable PostgreSQL | `true` |
| `postgresql.image.repository` | Image repository | `paradedb/paradedb` |
| `postgresql.image.tag` | Image tag | `v0.26.1-pg18` |
| `postgresql.persistence.enabled` | Enable persistence | `true` |
| `postgresql.persistence.size` | PVC size | `10Gi` |

### Redis

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `redis.enabled` | Enable Redis | `true` |
| `redis.image.repository` | Image repository | `redis` |
| `redis.image.tag` | Image tag | `8.10.2-alpine` |
| `redis.persistence.enabled` | Enable persistence | `true` |
| `redis.persistence.size` | PVC size | `1Gi` |

### Ingress

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class | `nginx` |
| `ingress.host` | Hostname | `ai-kb.example.com` |
| `ingress.tls.enabled` | Enable TLS | `false` |
| `ingress.tls.secretName` | TLS secret name | `""` |

### Secrets

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `secrets.dbUser` | Database username | `postgres` |
| `secrets.dbPassword` | Database password | `""` (required) |
| `secrets.dbName` | Database name | `ai_kb` |
| `secrets.redisUsername` | Redis ACL username (Redis 6.0+) | `""` |
| `secrets.redisPassword` | Redis password | `""` (required) |
| `secrets.jwtSecret` | JWT signing secret | `""` (required) |
| `secrets.systemAesKey` | 32-byte AES key for database field encryption | `""` (generated) |
| `secrets.existingSecret` | Use existing secret | `""` |

Set `secrets.systemAesKey` explicitly in production. When it is empty, the chart
generates a random key on first install and reuses it on upgrade by looking up
the existing Secret. If that Secret is ever deleted or recreated, a new key is
generated and data encrypted with the old one cannot be decrypted.

An `existingSecret` must contain `DB_USER`, `DB_PASSWORD`, `DB_NAME`,
`REDIS_USERNAME`, `REDIS_PASSWORD`, `JWT_SECRET` and `SYSTEM_AES_KEY`, plus
`NEO4J_USERNAME` and `NEO4J_PASSWORD` when `neo4j.enabled` is true.

### Data Files

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `dataFiles.persistence.enabled` | Persist uploaded files at `/data/files` | `true` |
| `dataFiles.persistence.size` | PVC size | `10Gi` |
| `dataFiles.persistence.existingClaim` | Use an existing PVC | `""` |

### Neo4j (GraphRAG)

Equivalent to the `neo4j` docker-compose profile. Enabling it sets `NEO4J_ENABLE=true` on the app.

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `neo4j.enabled` | Enable Neo4j | `false` |
| `neo4j.username` | Neo4j username | `neo4j` |
| `neo4j.password` | Neo4j password | `""` (required if enabled) |
| `neo4j.persistence.size` | PVC size | `10Gi` |

The chart has no MinIO or Qdrant components. Run them separately and configure
the app through `app.env` / `app.extraEnv` (`STORAGE_TYPE`, `RETRIEVE_DRIVER` and
the provider's variables).

### Ingress annotations

`ingress.annotations` defaults to ingress-nginx timeouts and
`proxy-body-size: 100m`. That body limit is independent of
`global.maxFileSizeMB`; raise both together.

## Security Best Practices

### Secret Management

**Never commit secrets to Git!** Use one of these approaches:

1. **Helm --set flags** (for testing)

   ```bash
   helm install ai-kb ./helm --set secrets.dbPassword=xxx
   ```

2. **External Secrets Operator** (recommended for production)

   ```yaml
   secrets:
     existingSecret: ai-kb-external-secret
   ```

3. **Sealed Secrets** (for GitOps)

   ```bash
   kubeseal < secret.yaml > sealed-secret.yaml
   ```

### Pod Security

The chart fits the `restricted` Pod Security Standard out of the box. Every pod runs as
non-root with the `RuntimeDefault` seccomp profile, and every container sets its user,
`allowPrivilegeEscalation: false`, drops all capabilities and runs on a read-only root
filesystem, with emptyDirs for the paths each image writes:

| Component | User | Port | Writable paths |
| ----------- | ------ | ------ | ---------------- |
| app | 1000 | 8080 | `/tmp`, the data-files volume |
| frontend | 101 | 8080 | `/tmp` (upload bodies are buffered here) |
| website | 101 | 8080 | `/tmp` |
| docreader | 1000 | 50051 | `/tmp` |
| postgresql | 999 | 5432 | the data volume, `/var/run/postgresql`, `/tmp` |
| redis | 999 | 6379 | the data volume |
| neo4j (optional) | 7474 | 7474, 7687 | the data volume, `/tmp`, `/logs`, `/var/lib/neo4j/{conf,plugins,import}` |

Override any of this through
`<component>.podSecurityContext` and `<component>.securityContext`.

## Upgrading

```bash
helm upgrade ai-kb ./helm \
  --namespace ai-kb \
  --reuse-values
```

The chart's PostgreSQL moved from `v0.18.9-pg17` to `v0.26.1-pg18`, a PostgreSQL major version. PostgreSQL 18 does not start on a PostgreSQL 17 data directory, so `pg_dumpall` from the old pod first and restore into the new one on an empty PVC. `--reuse-values` keeps a previously set `postgresql.image.tag`; pass `--set postgresql.image.tag=v0.26.1-pg18` or drop the override to pick up the new default.

## Uninstalling

```bash
helm uninstall ai-kb --namespace ai-kb

# Optional: Remove PVCs
kubectl delete pvc -n ai-kb -l app.kubernetes.io/instance=ai-kb
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n ai-kb
```

### View Logs

```bash
# Backend logs
kubectl logs -n ai-kb -l app.kubernetes.io/component=app -f

# Frontend logs
kubectl logs -n ai-kb -l app.kubernetes.io/component=frontend -f
```

### Common Issues

#### Pod stuck in Pending

- Check if PVCs are bound: `kubectl get pvc -n ai-kb`
- Verify storage class exists: `kubectl get sc`

#### Connection refused errors

- Wait for all pods to be Ready
- Check service endpoints: `kubectl get endpoints -n ai-kb`

#### Database connection errors

- Verify secrets are correct
- Check PostgreSQL logs: `kubectl logs -n ai-kb -l app.kubernetes.io/component=database`

## References

This Helm chart follows best practices from:

- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [ArgoCD Helm Chart](https://github.com/argoproj/argo-helm)
- [Prometheus Helm Charts](https://github.com/prometheus-community/helm-charts)
- [cert-manager Helm Chart](https://github.com/cert-manager/cert-manager)

## License

This chart is licensed under the MIT License - see the [LICENSE](https://github.com/jrmatherly/ai-kb/blob/main/LICENSE) file for details.
