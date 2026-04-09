---
name: dev.kind.setup
description: Create a Kind cluster and install the latest cert-manager.
---

# Kind Cluster Setup

Create a Kind (Kubernetes in Docker) cluster with cert-manager installed.

## Input

`$ARGUMENTS` contains an optional cluster name (default: `kind`).

## Steps

1. Run `kind create cluster --name <cluster-name>`. Use the name from `$ARGUMENTS` if provided, otherwise default to `kind`.
2. Wait for nodes to be `Ready`:
   ```
   kubectl wait --for=condition=Ready nodes --all --timeout=60s
   ```
3. Install the latest cert-manager:
   ```
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
   ```
4. Wait for cert-manager deployments to be available:
   ```
   kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=120s
   ```
5. Report the result: confirm cluster name, current kubectl context (`kubectl config current-context`), and cert-manager pod status (`kubectl get pods -n cert-manager`).
