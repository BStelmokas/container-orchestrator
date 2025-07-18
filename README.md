# Container Orchestration Platform

--

A lightweight, modular container orchestration platform built from scratch - à la mini Kubernetes - tailored for learning and experimentation.

The platform supports container management, service discovery, load balancing health monitoring, scaling, and a REST API.

--

# What It Does

This system automatically runs and manages containerized services. It can:

-   Start and stop Docker containers
-   Monitor container health and restart containers when needed
-   Register services for discovery and routing
-   Load balance HTTP traffic across healthy containers
-   Auto-scale services based on CPU usage
-   Deploy and manage multiple replicas of services
-   Interact via a REST API or CLI or web dashboard

--

# Technologies Used

Core Language: [Go 1.24](https://go.dev/)
Container Runtime: [Docker SDK v20.10](https://pkg.go.dev/github.com/docker/docker)
CLI Tooling: [Cobra](https://github.com/spf13/cobra)
Web Server: [Gin](https://github.com/gin-gonic/gin)
Networking: Native HTTP + Reverse Proxy
Project Layout: Modular Go structure

--

# Project Structure

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
│   └── main.go
├── dashboard
│   └── static
│   └── dashboard.html
├── go.mod
├── go.sum
├── internal
│   ├── manager
│   │   └── manager.go
│   ├── orchestrator
│   │   └── deployer.go
│   └── registry
│   ├── client.go
│   └── registry.go
└── orchestrator

--

# Features

### Container Manager

-   Start/stop containers via Docker
-   Auto health check via HTTP endpoints
-   Restarts unhealthy containers automatically

### Service Discovery

-   Built-in in-memory registry
-   Containers register themselves with name, IP, port
-   Lookup API for service routing

### Load Balancer

-   Reverse proxy HTTP traffic
-   Distributes requests using round-robin across healthy services

### Deployment Controller

-   Ensures `N` replicas are running for a service
-   Replaces dead containers
-   Generates unique names per replica

### Auto-Scaler

-   Monitors CPU usage of containers
-   Scales up when CPU > 80%
-   Scales down when CPU < 20%
-   Integrated into deployment controller

### REST API

-   `POST /deploy` - Deploy a service
-   `POST /scale/:name/:replicas` - Manually scale a service
-   `GET /status/:name` - Check container status
-   `GET /logs/:name` - View logs of service containers

### CLI

Command-line tool using Cobra:

```sh
containercli start --image nginx --name my-nginx
containercli stop my-nginx
containercli list
conatainercli status my-nginx
conatinercli version

```

### Web Dashboard

-   `GET /api/services` - Lists all containers
-   `POST /api/scale/:name/:delta` - Adjusts replica counts
-   `POST /api/restart/:name`- Redeploys containers of a service

--

# Setup Instructions

## Prerequisites

-   Docker daemon running
-   Go 1.24+
-   Ports 8000, 8080, and 9000 must be free

## Build & Run

Start the 3 main services in separate terminals:

// Start the service registry
go run cmd/registry/main.go

// Start the load balancer
go run cmd/loadbalancer/main.go

// Start the orchestrator
go run cmd/orchestrator/main.go

--

# API

Example deployment:
curl -X POST localhost:8080/deploy \
 -H "Content-Type: application/json" \
 -d '{"name":"nginx-service", "image":"nginx:latest", "replicas":3}'

Example scale:
curl -X POST localhost:8080/scale/nginx-service/5

Check status:
curl localhost:8080/status/nginx-service

--

# Design Philosophy

-   Modular: Each subsystem(CLI, registry, deployer) is isolated.
-   Distributed Concepts: Mirros real-world microservice coordination.
-   Educational: Built from first principles with clear comments and logs.
-   Resilient: Restarts dead containers, load balances HTTP traffic.

--

# Testing and Debugging

-   Logs output to terminal (health checks, deploy actions, scaling)
-   Health check interval: 30s
-   Auto-scaler interval: 15s
-   Dynamic container naming ensures unique deployments

--

# Example Use Case

-   Start nginx service with 3 replicas
-   Health checks ensure they stay alive
-   Load balancer routes requests to any of the 3
-   When CPU load increases, new containers are added
-   When load drops, containers are removed

--

© License
MIT License

--

# Author

Herodotus97
