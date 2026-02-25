# OCP User Audit Viewer

A complete auditing application for OpenShift clusters that captures kube-apiserver audit events for real users, stores them in PostgreSQL, and provides a web-based query interface with configurable access control.

## Architecture

```
Control Plane Nodes                    audit-system Namespace
┌───────────────────┐     ┌────────────────────────────────────────────┐
│ audit.log files   │     │                                            │
│        │          │     │  ┌──────────┐    ┌────────────────┐        │
│   audit-collector ─────────► Backend  ├────► PostgreSQL     │        │
│  (DaemonSet)      │     │  │ (Go+Gin) │    │ (30GB, daily   │        │
│                   │     │  └────┬─────┘    │  partitions)   │        │
└───────────────────┘     │       │          └────────────────┘        │
                          │  ┌────┴─────┐                              │
                          │  │ Frontend │◄── oauth-proxy sidecar       │
                          │  │ (React)  │    (OpenShift OAuth)         │
                          │  └──────────┘                              │
                          └────────────────────────────────────────────┘
```

## Components

| Component | Tech | Description |
|-----------|------|-------------|
| **Collector** | Go DaemonSet | Tails audit logs on control plane nodes, filters for real users and allowed verbs, batches events to the backend |
| **Backend** | Go + Gin | REST API for event ingestion, querying with filters/pagination, stats, retention management, and access control. Creates DB schema on startup |
| **Database** | PostgreSQL 16 | Stores audit events in a partitioned table (daily). 30GB retention with automatic oldest-partition cleanup |
| **Frontend** | React + Ant Design | Query UI with filters (username, verb, resource, namespace, client, exclude resources, time range), event detail view with JSON objects |
| **Auth** | OpenShift OAuth Proxy + ConfigMap | OAuth proxy handles authentication; backend enforces authorization via a ConfigMap listing allowed users and OpenShift groups |

## Prerequisites

1. An OpenShift 4.x cluster with cluster-admin access
2. `oc` CLI configured and logged in
3. `podman` for building images
4. **Audit policy** set to at least `WriteRequestBodies` to capture request/response objects:

```bash
oc patch apiserver cluster --type=merge -p '{"spec":{"audit":{"profile":"WriteRequestBodies"}}}'
```

> This causes a rolling restart of the API servers. Wait for it to complete before deploying.

## Quick Start

### 1. Configure

Set the `IMAGE_REGISTRY` environment variable to your container registry:

```bash
export IMAGE_REGISTRY=quay.io/myorg
```

This is **required** for all build, push, and deploy commands.

Edit the PostgreSQL password in `deploy/postgres/postgres.yaml` (the Secret's `POSTGRESQL_PASSWORD` and `DATABASE_URL` fields).

Generate and set a cookie secret for the oauth-proxy in `deploy/frontend/frontend.yaml`:

```bash
openssl rand -base64 32
```

### 2. Build and Push Images

```bash
make build-all push-all
```

Or with an explicit registry:

```bash
make build-all push-all IMAGE_REGISTRY=quay.io/myorg
```

### 3. Deploy

```bash
make deploy
```

The deploy target substitutes `IMAGE_REGISTRY` into the manifests automatically.

### 4. Access the UI

```bash
oc -n audit-system get route audit-frontend -o jsonpath='{.spec.host}'
```

Open the URL in your browser. You will be redirected to the OpenShift login page.

## Access Control

Access is controlled via the `audit-access-config` ConfigMap in the `audit-system` namespace. You can grant access by individual username or by OpenShift group.

### ConfigMap format

```yaml
# deploy/backend/access-config.yaml
data:
  access.yaml: |
    allowedUsers:
      - system:admin
      - kube:admin
    allowedGroups:
      - cluster-admins
```

- **`allowedUsers`**: List of individual usernames that are allowed access.
- **`allowedGroups`**: List of OpenShift Group CRs (from `user.openshift.io/v1`). The backend queries the OpenShift Groups API to resolve membership.

A user is granted access if they match **any** entry in either list.

### Updating access at runtime

Edit the ConfigMap directly — the backend watches for changes and reloads automatically (no restart needed):

```bash
oc -n audit-system edit configmap audit-access-config
```

Group membership is refreshed from the OpenShift API every 60 seconds.

### How it works

1. The **OAuth proxy** sidecar handles OpenShift authentication (any authenticated user can reach the app).
2. The **backend middleware** checks the authenticated user against:
   - The `allowedUsers` list (by username and email identity)
   - The `allowedGroups` list (resolved via the OpenShift Groups API)
3. Unauthorized users receive a `403 Forbidden` response.

## Filtering Behavior

### Collector Filters

**Included users**: All real human users, including `system:admin` and `kube:admin` (kubeadmin).

**Excluded**: Any username starting with `system:` (except `system:admin`), which covers service accounts, nodes, and internal components.

**Included verbs**: `create`, `update`, `patch`, `delete`, `get`, `list`.

**Excluded verbs**: `watch`, `deletecollection`, `proxy`, and any others.

### UI Filters

The frontend provides the following search filters:

| Filter | Type | Description |
|--------|------|-------------|
| Username | Free text | Partial match (case-insensitive) |
| Verb | Multi-select dropdown | create, update, patch, delete, get, list |
| Resource | Free text | Exact match (e.g. pods, deployments) |
| Namespace | Free text | Exact match |
| Resource name | Free text | Partial match (case-insensitive) |
| Response code | Number | HTTP status code (e.g. 200, 403) |
| Client | Multi-select + free text | Predefined: oc CLI, kubectl, Browser, OpenShift Console; also accepts custom values |
| Exclude resources | Multi-select + free text | Exclude specific resource types from results (e.g. events, endpoints) |
| Time range | Date/time picker | Start and end timestamps |

The **Client** column in the results table shows a color-coded tag indicating whether the action was performed via `oc CLI`, `kubectl`, `Browser`, or `OpenShift Console`, derived from the user agent string.

## Retention

- PostgreSQL data is stored in a 30GB PVC with daily table partitions
- A background goroutine runs hourly and:
  - Drops partitions older than 90 days
  - If total DB size exceeds 30GB, drops the oldest partitions until under the limit
- At least one partition is always retained

## Query API

The backend exposes a REST API for querying events:

```
GET /api/v1/events?username=john&verb=create,delete&resource=pods&namespace=default&from=2026-01-01T00:00:00Z&to=2026-02-01T00:00:00Z&page=1&page_size=50
```

| Parameter | Description |
|-----------|-------------|
| `username` | Partial match (case-insensitive) |
| `verb` | Comma-separated list: create, update, patch, delete, get, list |
| `resource` | Exact match: pods, deployments, configmaps, etc. |
| `namespace` | Exact match |
| `name` | Partial match (case-insensitive) |
| `from` / `to` | RFC3339 timestamps for time range |
| `response_code` | HTTP status code (e.g., 200, 403) |
| `client` | Comma-separated user agent substrings (e.g., `oc/,kubectl/`) |
| `exclude_resource` | Comma-separated resource types to exclude |
| `page` / `page_size` | Pagination (default: page=1, page_size=50) |
| `sort` | Field and direction (e.g., `timestamp DESC`) |

## Local Development

Run PostgreSQL locally:

```bash
podman run -d --name audit-pg -e POSTGRESQL_USER=audit -e POSTGRESQL_PASSWORD=audit -e POSTGRESQL_DATABASE=audit \
  -p 5432:5432 quay.io/sclorg/postgresql-16-c9s:latest
```

Run the backend (creates schema automatically on startup):

```bash
make dev-backend
```

Run the frontend (proxies API to localhost:8080):

```bash
make dev-frontend
```

## Project Structure

```
ocp-user-auditter/
├── collector/           # Audit log collector (DaemonSet)
│   ├── main.go
│   ├── filter/          # User + verb filtering
│   ├── tailer/          # File tailing logic
│   ├── sender/          # HTTP batch sender
│   └── Dockerfile
├── backend/             # REST API server
│   ├── main.go
│   ├── api/             # Gin handlers, middleware, access control
│   ├── db/              # PostgreSQL queries, migrations, retention
│   ├── models/          # Data structures
│   └── Dockerfile
├── frontend/            # React UI
│   ├── src/
│   │   ├── components/  # FilterBar, EventTable, JsonViewer
│   │   ├── pages/       # EventExplorer
│   │   ├── api/         # API client
│   │   └── types/       # TypeScript interfaces
│   ├── nginx.conf       # Reverse proxy config (forwards auth headers)
│   └── Dockerfile
├── deploy/              # OpenShift manifests
│   ├── namespace.yaml
│   ├── rbac/            # ServiceAccounts, ClusterRoles, ClusterRoleBindings
│   ├── postgres/        # StatefulSet, Service, Secret
│   ├── backend/         # Deployment, Service, access ConfigMap
│   ├── collector/       # DaemonSet
│   └── frontend/        # Deployment, Service, Route (with oauth-proxy sidecar)
├── Makefile
└── README.md
```
