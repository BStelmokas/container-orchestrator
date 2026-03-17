# Container Orchestration Platform

A lightweight, modular container orchestration platform built from scratch - a mini Kubernetes - tailored for learning and experimentation.

The platform supports container management, service discovery, load balancing, health monitoring, scaling, desired-state reconciliation, and a REST API.

# What It Does

This system automatically runs and manages containerized services. It can:

- Start and stop Docker containers
- Monitor container health and restart containers when needed
- Register services for discovery and routing
- Load balance HTTP traffic across healthy containers
- Deploy and manage multiple replicas of services
- Store desired service state and reconcile actual runtime state toward it
- Interact via a REST API or CLI or web dashboard

# Technologies Used

```text
Core Language: Go 1.24
Container Runtime: Docker SDK v20.10 / API v1.41 compatibility
CLI Tooling: Cobra
Web Server: Gin
Networking: Native HTTP + Reverse Proxy
Project Layout: Modular Go structure
```

# Project Structure

```
.
├── README.md
├── cmd
│   ├── cli
│   │   ├── cmd
│   │   │   ├── list.go
│   │   │   ├── root.go
│   │   │   ├── start.go
│   │   │   ├── status.go
│   │   │   ├── stop.go
│   │   │   └── version.go
│   │   └── main.go
│   ├── loadbalancer
│   │   └── main.go
│   ├── orchestrator
│   │   └── main.go
│   └── registry
│       └── main.go
├── dashboard
│   └── static
│       └── dashboard.html
├── go.mod
├── go.sum
├── internal
│   ├── domain
│   │   └── service.go
│   ├── manager
│   │   └── manager.go
│   ├── orchestrator
│   │   └── deployer.go
│   ├── registry
│   │   ├── client.go
│   │   ├── registry.go
│   │   └── types.go
│   └── state
│       └── store.go
└── orchestrator
```

# Features

### Container Manager

- Start/stop containers via Docker
- Auto health check via HTTP endpoints
- Restarts unhealthy containers automatically
- Removes stopped replicas during scale-down and replacement

### Service Discovery

- Built-in in-memory registry
- Replicas register under a logical service name
- Each replica is tracked as a distinct service instance
- Lookup APIs support both single-instance and multi-instance resolution

### Load Balancer

- Reverse proxy HTTP traffic
- Distributes requests using round-robin across healthy services
- Resolves all backends for a logical service through the registry

### Deployment Controller

- Ensures `N` replicas are running for a service
- Reconciles desired state against actual running containers
- Replaces missing replicas
- Generates unique names per replica

### Desired State Store

- Stores service definitions as first-class objects
- Keeps service name, image, and desired replica count together
- Acts as the source of truth for deploy, scale, restart, and delete operations

### REST API

- `POST /deploy` - Deploy a service (legacy-compatible endpoint)
- `POST /scale/:name/:replicas` - Manually scale a service (legacy-compatible endpoint)
- `GET /status/:name` - Check service status
- `GET /logs/:name` - View logs of service containers
- `GET /api/services` - List desired services with running container data
- `POST /api/services` - Create or update a service
- `POST /api/scale/:name/:delta` - Adjust replica counts
- `POST /api/restart/:name` - Restart all running replicas of a service
- `DELETE /api/services/:name` - Delete a service and remove its running replicas

### CLI

Command-line tool built with Cobra.

#### Run without installing (recommended for development)

```sh
go run ./cmd/cli start --image nginx --name my-nginx
go run ./cmd/cli stop my-nginx
go run ./cmd/cli list
go run ./cmd/cli status my-nginx
go run ./cmd/cli version
```

#### Build locally

Build the CLI binary:

```sh
go build -o containercli ./cmd/cli
```

Run the binary:

```sh
./containercli start --image nginx --name my-nginx
./containercli stop my-nginx
./containercli list
./containercli status my-nginx
./containercli version
```

#### Install globally (optional)

Install the binary into your system PATH:

```sh
go build -o /usr/local/bin/containercli ./cmd/cli
```

Run the binary:

```sh
containercli start --image nginx --name my-nginx
```

### Web Dashboard

- `GET /api/services` - Lists services, images, desired replicas, and running replicas
- `POST /api/scale/:name/:delta` - Adjusts replica counts
- `POST /api/restart/:name`- Redeploys containers of a service
- `DELETE /api/services/:name`- Deletes a service
- `http://localhost:8080/`

# Setup Instructions

## Prerequisites

- Docker daemon running
- Go 1.24+
- Ports 8000, 8080, and 9000 must be free

## Build & Run

Start the 3 main services in separate terminals:

Start the service registry

```sh
go run cmd/registry/main.go
```

Start the load balancer

```sh
go run cmd/loadbalancer/main.go
```

Start the orchestrator

```sh
go run cmd/orchestrator/main.go
```

# API

Example deployment:

```sh
curl -X POST localhost:8080/deploy \
 -H "Content-Type: application/json" \
 -d '{"name":"nginx-service", "image":"nginx:latest", "replicas":3}'
```

Example service creation through the newer API:

```sh
curl -X POST localhost:8080/api/services \
 -H "Content-Type: application/json" \
 -d '{"name":"nginx-service","image":"nginx:latest","replicas":3}'
```

Example scale:

```sh
curl -X POST localhost:8080/scale/nginx-service/5
```

Example dashboard-style scale adjustment:

```sh
curl -X POST localhost:8080/api/scale/nginx-service/1
```

Check status:

```sh
curl localhost:8080/status/nginx-service
```

Delete a service:

```sh
curl -X DELETE localhost:8080/api/services/nginx-service
```

# Design Philosophy

- Modular: Each subsystem (CLI, registry, desired-state store, deployer) is isolated.
- Control Plane Oriented: Services are stored as desired state and reconciled toward runtime state.
- Distributed Concepts: Mirrors real-world microservice coordination.
- Professional: Built from first principles with clear comments and logs.
- Resilient: Restarts dead containers and load balances HTTP traffic across replicas.

# Testing and Debugging

- Logs output to terminal (health checks, deploy actions, scaling)
- Health check interval: 30s
- Registry stores multiple instances per logical service
- Dynamic replica naming ensures unique deployments

# Example Use Case

- Start nginx service with 3 replicas
- Desired state is stored in the service store
- The deployment controller ensures 3 running replicas exist
- Health checks restart unhealthy containers
- The registry tracks all replicas under one logical service
- The load balancer routes requests to any of the 3 replicas

# License

MIT License

# Author

Benas Stelmokas
