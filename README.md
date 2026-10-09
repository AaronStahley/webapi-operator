# webapi-operator

A Kubernetes operator (Go, kubebuilder) that gives developers a small, stable API for running a containerized web API. A developer declares a `WebApi`; the operator creates and maintains the Deployment and Service behind it and reports status on the resource.

```yaml
apiVersion: platform.aaronstahley.dev/v1alpha1
kind: WebApi
metadata:
  name: podinfo
spec:
  image:
    repository: ghcr.io/stefanprodan/podinfo
    tag: 6.15.0
  containerPort: 9898
```

See [`config/samples`](config/samples) for a complete example.

## Status

Early-stage and under active development. Work in progress, not production-ready.

| Area | State |
|---|---|
| `WebApi` CRD (image, replicas, port, probes, resources, env, service, labels) | Done |
| Deployment + Service reconciliation with owner references | Done |
| `Ready` status condition, replica counts | Done |
| Namespace selector (operate only on labeled namespaces) | Done |
| Helm chart, CI (lint, unit, e2e, build, release) | Done |

## Development

Requires Go 1.25+, Docker, kubectl and [kind](https://kind.sigs.k8s.io/).

```bash
make manifests generate   # regenerate CRDs, RBAC and deepcopy after editing types
make test                 # unit tests (envtest)
make install              # install the CRD into the current cluster
make run                  # run the controller locally
kubectl apply -k config/samples/
```

This repository is one piece of a larger platform demo; the GitOps layer lives in the companion repositories.
