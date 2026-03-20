# Container Orchestrator

A small but real container orchestration platform written in Go.

This project implements the core control-plane ideas behind container orchestration systems in a compact, understandable codebase. It manages containerized services through a service-oriented API, maintains desired state, reconciles runtime state, observes health centrally, performs rolling restarts, persists service definitions, registers live replicas for discovery, and routes traffic across service backends through a load balancer.

The platform lets an operator declare what services should be running, and continuously reconciles the system toward that state.

The system is intentionally single-node and small in scope, but the architecture is real:

- **desired state**
- **reconciliation**
- **service identity**
- **service discovery**
- **centralized health observation**
- **generation-based rolling restart**
- **operator visibility through status, events, dashboard, and CLI**

---

## Try it in 60 seconds

Prerequisites:

- [Go](https://go.dev/) installed
- [Docker](https://www.docker.com/) installed and running

Clone the repository:

```bash
git clone https://github.com/BStelmokas/container-orchestrator
cd container-orchestrator
```

Start components (in separate terminals):

```bash
go run ./cmd/registry
go run ./cmd/loadbalancer
go run ./cmd/orchestrator
```

Then, in a fourth terminal, use the CLI:

```bash
go run ./cmd/cli deploy --name web --image nginx:latest --replicas 2
go run ./cmd/cli services
go run ./cmd/cli get web
go run ./cmd/cli scale web --replicas 4
go run ./cmd/cli restart web
go run ./cmd/cli events
go run ./cmd/cli delete web
```

Open the dashboard:

```
http://localhost:8080
```

This demonstrates the control-plane lifecycle: deploy → reconcile → scale → rolling restart → observe.

---

## Core capabilities

- Create and update services through a canonical HTTP API
- Persist desired service state to disk
- Reconcile running containers toward desired state
- Register multiple live replicas under one logical service
- Route traffic across replicas with round-robin load balancing
- Track container health centrally
- Replace unhealthy replicas through reconciliation
- Perform rolling restarts using service generations
- Expose service status, logs, and control-plane events
- Operate the platform through both a web dashboard and a CLI

---

## Architecture

### High-level flow

```text
                +----------------------+
                |   Orchestrator API   |
                +----------+-----------+
                           |
                           v
                +----------------------+
                |  Desired State Store |
                +----------+-----------+
                           |
                           v
                +----------------------+
                |  Reconciliation Loop |
                +----+-------------+---+
                     |             |
                     |             |
                     v             v
          +----------------+   +----------------+
          | Container      |   | Health Tracker |
          | Manager        |   |                |
          +-------+--------+   +--------+-------+
                  |                     |
                  |                     |
                  v                     |
          +----------------+            |
          | Running        |------------+
          | Containers     |  Health Observations
          +-------+--------+
                  |
                  v
          +----------------+
          | Service        |
          | Registry       |
          +-------+--------+
                  |
                  v
          +----------------+
          | Load Balancer  |
          +----------------+
```

## Component responsibilities

### Orchestrator API

The orchestrator API is the control-plane entry point.

It is responsible for:

- accepting service definitions and operations
- storing desired state
- triggering reconciliation
- serving service status
- exposing recent control-plane events
- serving the dashboard

**Location:**

- `cmd/orchestrator`

### Desired State Store

The desired state store is the source of truth for services.

It is responsible for:

- storing `ServiceSpec` objects in memory
- persisting them to disk
- loading them on startup
- tracking desired replica count and desired generation

**Location:**

- `internal/state`

### Reconciliation Loop

The reconciliation loop is the control-plane core.

It is responsible for:

- comparing desired state to actual running containers
- starting missing replicas
- removing extra replicas
- replacing unhealthy replicas
- progressing rolling restarts generation by generation

**Location:**

- `internal/orchestrator`

### Container Manager

The container manager is the runtime adapter.

It is responsible for:

- interacting with Docker
- starting containers
- stopping and removing containers
- applying container metadata such as service generation labels
- registering live backends in the service registry
- launching health monitors that report into the health tracker

**Location:**

- `internal/manager`

### Health Tracker

The health tracker is the centralized health state store.

It is responsible for:

- recording the latest health result per replica
- separating health observation from runtime mutation
- giving the reconciler a consistent source for health-based replacement decisions
- feeding status reporting

**Location:**

- `internal/health`

### Service Registry

The registry maps one logical service to many concrete instances.

It is responsible for:

- registering service instances
- deregistering specific instances
- returning all backends for a service
- acting as the discovery source for the load balancer

**Locations:**

- `cmd/registry`
- `internal/registry`

### Load Balancer

The load balancer is the traffic router.

It is responsible for:

- resolving a logical service into its registered backends
- applying round-robin selection
- proxying requests to the selected backend

**Location:**

- `cmd/loadbalancer`

### Status Builder

The status builder creates the service view exposed to operators.

It combines:

- desired state
- runtime state
- health state
- rollout state

It reports:

- desired replicas
- running replicas
- healthy replicas
- unhealthy replicas
- unknown replicas
- outdated replicas
- desired generation
- overall service condition

**Location:**

- `internal/status`

### Dashboard

The dashboard provides a lightweight operator interface.

It is responsible for:

- listing services
- showing replica health and rollout state
- showing recent control-plane events
- allowing scale, restart, and delete actions

**Location:**

- `dashboard/static/dashboard.html`

### CLI

The CLI is the primary operator interface and communicates exclusively with the orchestrator API.

It is responsible for:

- deploying services
- listing services
- querying service status
- scaling services
- requesting rolling restarts
- deleting services
- viewing recent control-plane events

**Location:**

- `cmd/cli`

## Service lifecycle

### Create or update a service

1. A client sends `POST /api/services`
2. The service spec is validated
3. The desired state store saves the spec
4. The desired state file is written to disk
5. The reconciler compares desired state to actual state
6. Missing replicas are started
7. Replicas register themselves in the service registry
8. Health monitors begin reporting observations
9. Status and events become visible to operators

### Scale a service

1. A client sends `POST /api/services/:name/scale`
2. The desired replica count is updated in the store
3. The desired state file is rewritten
4. The reconciler adds or removes replicas until runtime matches desired state

### Rolling restart a service

1. A client sends `POST /api/services/:name/restart`
2. The service generation is incremented
3. The updated desired state is persisted
4. The reconciler creates new-generation replicas
5. The reconciler removes outdated replicas gradually
6. Service status reports `rolling` until outdated replicas are gone

### Replace an unhealthy replica

1. A health monitor probes a running replica
2. A failure is recorded in the health tracker
3. During reconciliation, the unhealthy replica is identified
4. The reconciler removes it
5. The reconciler starts a replacement if capacity is missing

## Canonical API

### List services

`GET /api/services`

Returns all services with centralized status information.

### Get one service

`GET /api/services/:name`

Returns one service with full status.

### Create or update a service

`POST /api/services`

Example body:

```json
{
  "name": "web",
  "image": "nginx:latest",
  "replicas": 3
}
```

### Scale a service

`POST /api/services/:name/scale`

Example body:

```json
{
  "replicas": 5
}
```

### Request a rolling restart

`POST /api/services/:name/restart`

### Delete a service

`DELETE /api/services/:name`

### Get logs for a service

`GET /api/services/:name/logs`

### Get recent control-plane events

`GET /api/events`

## CLI usage

The CLI is a control-plane client.

### Deploy a service

```bash
go run ./cmd/cli deploy --name web --image nginx:latest --replicas 3
```

### List services

```bash
go run ./cmd/cli services
```

### Get one service

```bash
go run ./cmd/cli get web
```

### Scale a service

```bash
go run ./cmd/cli scale web --replicas 5
```

### Request a rolling restart

```bash
go run ./cmd/cli restart web
```

### Delete a service

```bash
go run ./cmd/cli delete web
```

### Show recent events

```bash
go run ./cmd/cli events
```

### Target a different orchestrator API server

```bash
go run ./cmd/cli services --server http://localhost:8080
```

## Service state and persistence

Desired service definitions are persisted to:

```
data/services.json
```

This file is part of the system’s native runtime contract. Persisted service specs are expected to match the current format, including a valid positive service generation.

On startup:

- the orchestrator loads the saved desired state
- the reconciler uses that desired state as its source of truth
- running containers are converged toward that state

This means a process restart does not erase the services the system is meant to maintain.

## Status model

Each service reports:

- `desiredReplicas`
- `runningReplicas`
- `healthyReplicas`
- `unhealthyReplicas`
- `unknownReplicas`
- `outdatedReplicas`
- `generation`
- `overallStatus`

Each replica reports:

- runtime status
- health status
- generation
- whether it is outdated relative to the desired generation
- last health error when applicable

### Overall status meanings

- `progressing` — running replicas are still below desired capacity
- `rolling` — outdated replicas are still live during a generation transition
- `degraded` — one or more replicas are explicitly unhealthy
- `starting` — replicas exist but some have not yet completed health checks
- `healthy` — service is fully converged, healthy, and current

## Event model

The platform records recent control-plane activity in memory.

Examples include:

- service writes through the API
- scale requests
- rolling restart requests
- replica creation and removal
- health failures
- reconcile-driven replacement actions

Events are exposed through:

```
GET /api/events
```

and surfaced in the dashboard and CLI.

## Testing

Run the full test suite with:

```bash
go test ./...
```

Current smoke coverage includes:

- desired-state persistence round-trip
- multi-instance registry semantics
- rollout-aware status computation

## Design principles

### Service-oriented control plane

The platform manages **services**, not ad hoc container actions. Users declare the desired service state, and the control plane works to realize it.

### Reconciliation owns mutation

Health checking does not restart containers directly. Observers report state, and the reconciler decides how runtime state should change.

### Logical service identity is distinct from replica identity

A service such as `web` may have many replicas. Discovery, status, and load balancing all operate on the logical service, not on individual container names.

### Rolling restart is a desired-state change

Restart is implemented by bumping the desired generation, not by stopping everything at once.

### Operator visibility matters

The platform exposes health state, rollout state, recent events, and replica details so behavior is inspectable and debuggable.

## Scope and limitations

This project is intentionally focused and single-node.

Current scope limits include:

- single-host orchestration
- Docker-only runtime backend
- in-memory health state
- in-memory event history
- no authentication or RBAC
- no distributed consensus or leader election
- no multi-node scheduling
- no persistent historical metrics pipeline
- no advanced production autoscaling system in the current codebase

These are scope choices, not architectural accidents.

## Project goal

This repository is meant to demonstrate the ability to design and implement a coherent small control plane with real orchestration ideas:

- desired state
- reconciliation
- service discovery
- health-driven replacement
- rolling updates
- status reporting
- operator tooling

It is compact by design, but the system model is real.

---
