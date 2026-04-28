---
name: tools.kubectl
description: kubectl/oc CLI patterns for token-efficient Kubernetes operations. Triggers when running kubectl, oc, or Kubernetes API commands.
user-invocable: false
---

# kubectl / oc Guidelines

Always minimize output. Never dump full resources. Use jq as the only external processing tool — no python, awk, or grep on kubectl output.

## Output format decision tree

| Need | Format | Example |
|------|--------|---------|
| Existence check | `-o name --ignore-not-found` | `oc get pod foo -o name --ignore-not-found` |
| One field | `-o jsonpath='{.path}'` | `oc get pod foo -o jsonpath='{.status.phase}'` |
| 2-4 fields, tabular | `-o custom-columns=... --no-headers` | see below |
| Complex filter/sort/count | `-o json \| jq` | see jq recipes |
| What would change | `kubectl diff -f` | `kubectl diff -f manifest.yaml` |
| Wait for condition | `kubectl wait --for=...` | `oc wait --for=condition=Ready pod/foo --timeout=120s` |

## Server-side filtering

Always filter before the data reaches the client.

```bash
# label selectors — most versatile, works on all resources
oc get pods -l app=myapp,version=v2
oc get pods -l 'app in (frontend,backend)'
oc get pods -l '!canary'

# field selectors — limited but useful
# pods support: status.phase, spec.nodeName, spec.restartPolicy, spec.serviceAccountName
# most other resources: only metadata.name, metadata.namespace
oc get pods --field-selector status.phase!=Running
oc get pods -A --field-selector spec.nodeName=worker-3

# combine both
oc get pods -l app=web --field-selector status.phase=Running

# prefer -n over --field-selector metadata.namespace=X
oc get pods -n myproject    # not: --field-selector metadata.namespace=myproject
```

## Custom columns

Best for 2-4 fields in readable tabular output:

```bash
oc get pods -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,NODE:.spec.nodeName --no-headers

oc get deployments -o custom-columns=NAME:.metadata.name,READY:.status.readyReplicas,DESIRED:.spec.replicas --no-headers

oc get crd -o custom-columns=NAME:.metadata.name,GROUP:.spec.group,SCOPE:.spec.scope --no-headers
```

## jsonpath quick reference

Use for 1-2 field extractions without jq:

```bash
# single value
oc get pod foo -o jsonpath='{.status.phase}'

# range iteration
oc get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}'

# filter expression
oc get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}'

# condition status
oc get pod foo -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

**Limitations**: no regex, no arithmetic, no sorting, no grouping. Switch to jq for those.

## jq recipes

Always output as `@tsv` for compact tabular data.

```bash
# pod status summary
oc get pods -A -o json | jq -r '
  .items[] | [.metadata.namespace, .metadata.name, .status.phase] | @tsv'

# non-ready pods only
oc get pods -A -o json | jq -r '
  .items[] | select(.status.phase != "Running") |
  [.metadata.namespace, .metadata.name, .status.phase] | @tsv'

# pods with highest restart counts (top 10)
oc get pods -A -o json | jq -r '
  [.items[] | {ns: .metadata.namespace, name: .metadata.name,
   restarts: ([.status.containerStatuses[]?.restartCount] | add // 0)}]
  | sort_by(-.restarts) | .[:10][] | [.ns, .name, .restarts] | @tsv'

# unique container images
oc get pods -A -o json | jq -r '[.items[].spec.containers[].image] | unique[]'

# resource count by namespace
oc get pods -A -o json | jq '
  .items | group_by(.metadata.namespace) |
  map({ns: .[0].metadata.namespace, count: length}) | sort_by(-.count)'

# CRD instance conditions (e.g., InferenceService readiness)
oc get inferenceservices -A -o json | jq -r '
  .items[] | [.metadata.namespace, .metadata.name,
  (.status.conditions[]? | select(.type=="Ready") | .status)] | @tsv'

# CRD inventory by API group
oc get crd -o json | jq -r '
  .items[] | [.metadata.name, .spec.group, (.spec.versions[].name | tostring)] | @tsv'

# operator versions (CSV)
oc get csv -A -o json | jq -r '
  .items[] | [.metadata.namespace, .metadata.name, .spec.version] | @tsv'
```

## Batch and efficiency patterns

```bash
# get multiple specific resources in one call
oc get pod foo bar baz -o custom-columns=NAME:.metadata.name,STATUS:.status.phase --no-headers

# delete by label, not loop
oc delete pods -l app=test

# apply a directory
oc apply -f ./manifests/

# diff before apply (exit code 0 = no diff, 1 = has diff)
kubectl diff -f manifest.yaml

# wait instead of poll loop
oc wait --for=condition=Ready pod -l app=myapp --timeout=120s
oc wait --for=delete pod/foo --timeout=60s

# check CRD existence
oc get crd inferenceservices.serving.kserve.io -o name --ignore-not-found

# check API group availability
oc api-resources --api-group=serving.kserve.io --no-headers
```

## oc-specific commands

```bash
# context verification (always do first)
oc whoami
oc whoami --show-server
oc project

# route URLs
oc get routes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.host}{"\n"}{end}'

# route admission status
oc get route myroute -o jsonpath='{.status.ingress[0].conditions[0].status}'

# cluster version
oc get clusterversion -o jsonpath='{.items[0].status.desired.version}'

# cluster operator health (compact)
oc get co -o custom-columns=NAME:.metadata.name,AVAIL:.status.conditions[?(@.type=="Available")].status,DEGR:.status.conditions[?(@.type=="Degraded")].status --no-headers

# switch project (shorter than kubectl config set-context)
oc project myproject
```

## Anti-patterns — never do these

| Do not | Why | Do instead |
|--------|-----|------------|
| `-o yaml` | Hundreds of lines per resource | `-o jsonpath` or `-o json \| jq` for specific fields |
| `kubectl describe` | Unstructured text, unparseable | `-o json \| jq` for conditions, events, status |
| `kubectl get pods \| grep Running` | Fragile text parsing | `--field-selector status.phase=Running` |
| Loop: `for p in $(oc get pods -o name); do oc get $p -o json; done` | N+1 queries | Single `oc get pods -o json \| jq` |
| `kubectl get all -A` | Unbounded, returns everything | Query specific resource types with selectors |
| `kubectl api-resources` (unfiltered) | ~100 lines | `--api-group=apps` or `--namespaced=true` |
| `kubectl logs pod` (no tail) | Unbounded output | `--tail=50` or `--since=5m` |
| `grep` / `awk` on kubectl output | Fragile, columns shift | jq or jsonpath |
