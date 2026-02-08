---
name: testcontainers
description: Use this skill when running tests that use testcontainers, when setting up Podman for testcontainers, when troubleshooting container-based tests, or when the user mentions "testcontainers", "Podman tests", "Docker tests", or "container engine"
---

# Testcontainers Environment Setup

This skill provides guidance on running testcontainers-based tests with proper environment configuration for Docker or Podman.

## Container Engine Detection

Before running tests, detect which container engine is available:

```bash
# Check for Docker
docker info >/dev/null 2>&1 && echo "Docker available"

# Check for Podman
podman info >/dev/null 2>&1 && echo "Podman available"

# Check DOCKER_HOST environment variable
echo "DOCKER_HOST=${DOCKER_HOST:-not set}"
```

## Docker Configuration

Docker typically works out of the box with testcontainers. Standard setup:

- Docker Desktop or Docker Engine must be running
- No special environment variables required
- Ensure the user has permissions to access the Docker socket

```bash
# Verify Docker is working
docker run --rm hello-world
```

## Podman Configuration

Podman requires specific environment variables for testcontainers compatibility:

### Required Environment Variables

```bash
# Set DOCKER_HOST to the Podman socket
export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')

# Disable Ryuk (cleanup container) - Podman doesn't support it well
export TESTCONTAINERS_RYUK_DISABLED=true
```

### One-liner for Running Tests

```bash
DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}') \
TESTCONTAINERS_RYUK_DISABLED=true \
go test ./...
```

### Podman Requirements

- **Podman 4.1+** is required for `host-gateway` support
- Podman machine must be running (macOS/Windows)
- Uses `host.containers.internal:host-gateway` for container-to-host communication

### Verify Podman Setup

```bash
# Check Podman machine is running
podman machine list

# Verify socket path
podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}'

# Test container execution
podman run --rm hello-world
```

## Language-Specific Notes

### Go (testcontainers-go)

```go
import "github.com/testcontainers/testcontainers-go"

// testcontainers-go automatically detects DOCKER_HOST
// No code changes needed for Podman compatibility
```

For Podman with host networking, use `CustomizeRequest` with `ExtraHosts`:

```go
req.CustomizeRequest = func(req testcontainers.GenericContainerRequest) error {
    req.HostConfigModifier = nil // Don't overwrite existing modifiers
    req.ExtraHosts = []string{"host.containers.internal:host-gateway"}
    return nil
}
```

### Java (testcontainers-java)

```java
// Environment variables are read automatically
// Ensure DOCKER_HOST and TESTCONTAINERS_RYUK_DISABLED are set
```

### Python (testcontainers-python)

```python
# Uses DOCKER_HOST environment variable automatically
import testcontainers
```

## Troubleshooting

### Docker Issues

| Problem | Solution |
|---------|----------|
| `Cannot connect to Docker daemon` | Start Docker Desktop or Docker Engine |
| `Permission denied` | Add user to `docker` group: `sudo usermod -aG docker $USER` |
| `docker.sock not found` | Verify Docker is running and socket exists |

### Podman Issues

| Problem | Solution |
|---------|----------|
| `Cannot connect to Podman` | Start Podman machine: `podman machine start` |
| `Socket not found` | Check socket path: `podman machine inspect` |
| `Ryuk container fails` | Set `TESTCONTAINERS_RYUK_DISABLED=true` |
| `host-gateway not supported` | Upgrade to Podman 4.1+ |
| `Network issues` | Use `host.containers.internal:host-gateway` in ExtraHosts |

### Common Environment Issues

```bash
# Verify environment is set correctly
env | grep -E "(DOCKER_HOST|TESTCONTAINERS)"

# Test socket connectivity
curl --unix-socket ${DOCKER_HOST#unix://} http://localhost/_ping
```

## Quick Reference

### Docker

```bash
# Usually no setup needed
go test ./...
```

### Podman

```bash
export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')
export TESTCONTAINERS_RYUK_DISABLED=true
go test ./...
```
