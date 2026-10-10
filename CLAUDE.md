# CLAUDE.md: webapi-operator

Kubernetes operator (Go, kubebuilder) for the `WebApi` CRD. A developer declares image, port, probes, resources, env, service and rollout settings; the controller creates and owns a Deployment and Service, and reports `Ready` / `Progressing` / `Degraded` conditions.

## Layout

- `api/v1alpha1/webapi_types.go`: the API. Kubebuilder markers drive CRD schema, CEL validation and printer columns.
- `internal/controller/`: reconciler (`webapi_controller.go`), rollout mapping (`rollout.go`), conditions (`status.go`), envtest suites.
- `config/`: kustomize output from controller-gen (CRD, RBAC) plus samples.
- `helm/webapi-operator/`: the chart Argo CD deploys. `crds/` holds a **copy** of the generated CRD.
- `.github/workflows/`: lint, test, e2e, build; `release.yml` pushes `ghcr.io/aaronstahley/webapi-operator` on `v*` tags.

## Build, test, run

Requires Go (version in `go.mod`), Docker, kubectl, kind.

```bash
make manifests generate        # after any change to api/ or RBAC markers
cp config/crd/bases/platform.aaronstahley.dev_webapis.yaml helm/webapi-operator/crds/
make test                      # fmt, vet, envtest unit tests
make build                     # bin/manager
make lint
make install run               # CRD into current cluster, controller locally
kubectl apply -k config/samples/
make docker-build IMG=...      # image
```

The Helm CRD copy must match `config/crd/bases/` byte for byte; check it with `diff` before committing.

## Release

Tag `vX.Y.Z` on `main`. The release workflow tests, builds and pushes to GHCR. `platform-gitops/manifests/webapi-operator*/values.yaml` pin the tag (Kargo's `infrastructure` project promotes it).

## The five-repo platform

| Repo | Role |
|---|---|
| `webapi-operator` | Go/kubebuilder operator. Owns the `WebApi` CRD (`platform.aaronstahley.dev/v1alpha1`) and reconciles it into a Deployment and Service. Ships a Helm chart and a GHCR image. |
| `platform-gitops` | Platform team repo. Argo CD app-of-apps (`apps/infrastructure`, `apps/domains`), AppProjects as tenant boundaries, namespaces, Kargo, operator instances per env, docs. |
| `platform-bootstrap` | Day-0: kind cluster, Argo CD install, credentials, root app-of-apps. `make all`. |
| `payments-gitops` | Domain repo: podinfo as a `WebApi`, Kustomize overlays per env, Kargo warehouse and stages. |
| `orders-gitops` | Same shape as payments, for the orders domain. |

Flow: a developer changes a `WebApi` in a domain repo on Git. Kargo turns the change into Freight and promotes dev -> qa -> prod by rendering into `stage/<env>` branches. Argo CD syncs each env from its branch, and the operator reconciles the `WebApi`.

## Conventions (all repos)

- Commit as `Aaron Stahley <AaronStahley@users.noreply.github.com>` (set via local `git config`).
- Keep commits small and meaningful.
- Never hand-edit generated files (deepcopy, CRDs, RBAC). In webapi-operator run `make manifests generate`, then refresh the Helm chart's CRD copy in `helm/webapi-operator/crds/`.
- Demo workload is podinfo: `ghcr.io/stefanprodan/podinfo` `6.15.0`, port `9898`, liveness `/healthz`, readiness `/readyz`.
- Domains are `payments` and `orders`. CRD group/version is `platform.aaronstahley.dev/v1alpha1`.
- These repos are public. No secrets, tokens or personal email addresses.
- Run tests and the build after changes, and only commit/push when they pass.
