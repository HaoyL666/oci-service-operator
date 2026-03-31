# Codemap — OCI Service Operator

Quick-reference for agents. Read this instead of exploring the repo.

## Architecture

```
main.go                                    ← Manager bootstrap; iterates group registrations
├── api/<group>/v1beta1/*_types.go         ← CRD Spec/Status definitions (grouped by OCI API group)
├── controllers/<group>/*_controller.go    ← Reconcile loops, mostly generated
├── internal/registrations/*_generated.go  ← Generated scheme/controller wiring
├── internal/registrations/manual_groups.go← Handwritten registration overrides
├── pkg/core/reconciler.go                 ← BaseReconciler — shared reconciliation logic
├── pkg/servicemanager/<group>/<resource>/ ← OCI API client wrappers per resource
│   ├── *_serviceclient.go                 ← OCI SDK interface or generated client wrapper
│   ├── *_servicemanager.go                ← CreateOrUpdate / Delete / GetCrdStatus entrypoint
│   └── *_secretgeneration.go              ← Secret payload after ACTIVE state where needed
├── pkg/validator/                       ← API spec + SDK parity validator
├── pkg/config/                          ← OSOK + user auth configuration
├── pkg/credhelper/                      ← Kubernetes secret-based credential helper
├── pkg/authhelper/                      ← OCI auth config provider
├── formal/                              ← Runtime intent, provider-fact imports, logic gaps, and generated diagrams
└── config/                              ← Generated CRD YAML, RBAC, samples
```

## Key Interface

```go
// pkg/servicemanager/interfaces.go
type OSOKServiceManager interface {
    CreateOrUpdate(ctx, obj, req) (OSOKResponse, error)
    Delete(ctx, obj) (bool, error)
    GetCrdStatus(obj) (*OSOKStatus, error)
}
```

Every service manager implements this. Controllers call it via `core.BaseReconciler`.

## API Grouping

Types are organized by OCI service group, NOT flat in `api/v1beta1/`:

```
api/
├── database/v1beta1/        ← autonomousdatabases_types.go, webhook
├── streaming/v1beta1/       ← stream_types.go
├── mysql/v1beta1/           ← mysqldbsystem_types.go
├── queue/v1beta1/           ← queue_types.go, channel, message, stats, workrequest types
├── core/v1beta1/            ← ~130 compute/networking types (generated)
├── identity/v1beta1/        ← ~40 IAM types (generated)
├── objectstorage/v1beta1/   ← ~16 object storage types (generated)
├── loadbalancer/v1beta1/    ← ~20 LB types (generated)
├── dns/v1beta1/             ← ~13 DNS types (generated)
├── functions/v1beta1/       ← 5 function types (generated)
└── ... (30+ service groups total, mostly generated CRD types)
```

CRD names follow `<service>.oracle.com` pattern (e.g. `database.oracle.com_autonomousdatabases.yaml`).

## Controller Grouping

Controllers are organized by service subdirectory:

```
controllers/
├── core/subnet_controller.go
├── database/autonomousdatabases_controller.go
├── streaming/stream_controller.go
├── mysql/mysqldbsystem_controller.go
└── ... many other generated controllers
```

Controllers are not hand-wired in `main.go` anymore. Startup goes through the registrations package:

```go
for _, registration := range registrations.All() {
    _ = registration.SetupWithManager(registrationContext)
}
```

## Reconcile Logic Status

Most API groups now have generated controller + service-manager scaffolds. For actual service-manager reconcile behavior, use these references:

| Status | Reference | Notes |
|--------|-----------|------|
| Handwritten logic | `pkg/servicemanager/streams/` | Main reference for stream reconcile/update/secret flow |
| Handwritten logic | `pkg/servicemanager/mysql/dbsystem/` | Main reference for MySQL DBSystem reconcile flow |
| Generated scaffold | `pkg/servicemanager/core/subnet/` | Typical generated structure and delegation pattern |

Outside those two handwritten implementations, most service managers are currently placeholders/scaffolds waiting for fuller runtime logic.

## Implementing Real Reconciler Logic

For most generated groups, the controller and registration layers already exist and only delegate into `core.BaseReconciler` plus a service-manager factory. The main handwritten runtime seam is usually:

`pkg/servicemanager/<group>/<resource>/`

Default reading order for a new real reconciler implementation:

1. `api/<group>/v1beta1/<resource>_types.go` ← understand Spec + Status surface
2. `controllers/<group>/<resource>_controller.go` ← confirm whether the controller is only BaseReconciler delegation
3. `internal/registrations/<group>_generated.go` ← confirm which service manager is wired
4. vendored OCI SDK under `vendor/github.com/oracle/oci-go-sdk/v65/...` ← branch-local source of truth for current SDK field/operation surface
5. `pkg/servicemanager/<group>/<resource>/` ← implement or override the real runtime behavior here
6. `formal/controller_manifest.tsv` row + `formal/controllers/<service>/<slug>/spec.cfg` + `logic-gaps.md` + `formal/imports/<service>/<slug>.json` + `formal/controllers/<service>/<slug>/diagrams/runtime-lifecycle.yaml` ← when present, treat these as the per-resource runtime contract and promotion metadata
7. `oracle/terraform-provider-oci` source ← optional secondary reference when you need deeper CRUD, wait, datasource, or field-handling patterns beyond the repo-local formal summary

Generated service-manager packages usually provide:

- a thin `*_servicemanager.go` adapter
- a generated `*_serviceclient.go` with `WithClient(...)` or equivalent handwritten extension seam
- default generic CRUD/status behavior via `pkg/servicemanager/generatedruntime/`

If a file starts with `// Code generated by generator. DO NOT EDIT.` or `// Code generated by controller-gen. DO NOT EDIT.`, treat it as generator-owned. Do not hand-edit that file unless the task is explicitly about changing generator output or the generator contract itself. Prefer these paths instead:

- edit the generator source or source-of-truth config
- add manual logic in a separate non-generated file in the same package when an extension seam exists
- use manual carve-out files such as webhooks, tests, or handwritten service-manager implementations

Only plan controller or registration edits when the resource needs something beyond the default path, such as:

- custom watches or predicates
- extra RBAC markers
- non-default factory wiring
- manual group bridging

For a generated resource like `functions/application`, the useful trace is:

`formal/...` intent → generated controller/registration wiring → `pkg/servicemanager/functions/application/` implementation seam

Field ownership and omission decisions should follow this order:

1. vendored OCI SDK in this repo
2. checked-in repo contracts and formal metadata
3. Terraform provider as a secondary runtime reference

Do not reject an implementation because a field exists in a newer SDK copy outside this repo unless the task is explicitly about upgrading the pinned SDK or refreshing the generated contract for that newer version.

## Service Manager Files

```
pkg/servicemanager/
├── interfaces.go                                    ← OSOKServiceManager interface
├── core/subnet/
│   ├── subnet_serviceclient.go                      ← Generated client interface/runtime
│   └── subnet_servicemanager.go                     ← Generated scaffold manager
├── mysql/dbsystem/
│   ├── dbsystem_serviceclient.go                    ← OCI SDK interface
│   ├── dbsystem_servicemanager.go                   ← Handwritten reconcile logic
│   ├── dbsystem_servicemanager_test.go              ← Tests
│   ├── dbsystem_secretgeneration.go                 ← Secret payload
│   └── export_test.go                               ← Test helpers
├── streams/
    ├── stream_serviceclient.go                      ← OCI SDK interface
    ├── stream_servicemanager.go                     ← Handwritten reconcile logic
    ├── stream_servicemanager_test.go                ← Tests
    └── stream_secretgeneration.go                   ← Secret payload
└── <many other group/resource dirs>/                ← Mostly generated scaffold pairs
```

## Validator Package

```
pkg/validator/
├── run.go                 ← Main entry point
├── upgrade_runner.go      ← Upgrade compatibility checks
├── allowlist/             ← Allowlist config parsing
├── apispec/               ← API spec analysis + registry
├── config/                ← Validator options
├── diff/                  ← Diff building between specs
├── provider/              ← Provider analysis
├── report/                ← Report rendering
├── sdk/                   ← SDK parity analysis + registry
└── upgrade/               ← Upgrade compatibility analysis
```

See `docs/validator-guide.md` for usage.

## Reference Patterns (use as templates)

| Pattern | Best Reference | Why |
|---------|---------------|-----|
| Simple CRD types | `api/streaming/v1beta1/stream_types.go` | Minimal fields, clean markers |
| Simple service manager | `pkg/servicemanager/streams/` | Main handwritten reconcile reference |
| Generated service manager scaffold | `pkg/servicemanager/core/subnet/` | Current generated directory shape |
| Controller | `controllers/core/subnet_controller.go` | Typical BaseReconciler delegation |
| Secret generation | `pkg/servicemanager/mysql/dbsystem/dbsystem_secretgeneration.go` | GetCredentialMap pattern |
| Tests | `pkg/servicemanager/mysql/dbsystem/dbsystem_servicemanager_test.go` | Current handwritten logic tests |
| RBAC roles | `config/rbac/stream_editor_role.yaml` | Minimal role template |
| Sample manifest | `config/samples/oci_v1beta1_stream.yaml` | Simple sample |
| Docs | `docs/oss.md` | Simple service documentation |

## File Naming Conventions

```
api/<group>/v1beta1/<resource>_types.go               ← CRD types (grouped by API group)
controllers/<group>/<resource>_controller.go          ← Controller (grouped by API group)
pkg/servicemanager/<group>/<resource>_serviceclient.go← OCI client interface or generated client
pkg/servicemanager/<group>/<resource>_servicemanager.go← Manager entrypoint
pkg/servicemanager/<group>/<resource>_secretgeneration.go ← Secret payload when applicable
pkg/servicemanager/<group>/<resource>_servicemanager_test.go ← Tests for handwritten logic
pkg/servicemanager/<group>/<resource>/export_test.go  ← Test helpers
config/samples/oci_v1beta1_<resource>.yaml            ← Sample manifest
config/rbac/<resource>_editor_role.yaml               ← RBAC
docs/<service>.md                                     ← Documentation
```

## Key Patterns (MUST follow)

1. **Service-manager client seam** — Handwritten managers commonly inject `ociClient <Interface>`; generated managers usually expose a `client <Interface>` field plus `WithClient(...)` as the handwritten extension seam
2. **Lifecycle states** — FAILED → set failed status; ACTIVE → set active; other → requeue
3. **Conditional fields** — `if spec.X != "" { details.X = common.String(spec.X) }` — never send zero-values
4. **Secret generation** — `GetCredentialMap()` returns `map[string]string` after ACTIVE
5. **Registration** — Controller and scheme wiring goes through `internal/registrations/`, with `main.go` iterating `registrations.All()`
6. **Generated files** — Always commit `zz_generated.deepcopy.go` + CRD YAML after `make generate && make manifests`

## Webhooks

Only Autonomous DB has a webhook:
- `api/database/v1beta1/autonomousdatabases_webhook.go`

## Build Commands

```bash
go build ./...          # Build
go test ./...           # Tests
go vet ./...            # Static analysis
make generate           # Regenerate deepcopy (after *_types.go changes)
make manifests          # Regenerate CRD YAML (after *_types.go changes)
```

## Agent Infrastructure

```
agnts/
├── launch.sh              ← Tmux agent launcher (all/multi/planner/coder/reviewer)
├── watchdog.sh            ← Agent health monitoring
├── plans/                 ← Draft plans written by planner_draft
│   └── .gitkeep
└── roles/                 ← Role instructions per agent
    ├── planner-single.md  ← Single planner (designs + creates beads directly)
    ├── planner.md         ← Multi-agent planner orchestrator
    ├── planner_draft.md   ← Design sub-agent (writes plan files)
    ├── planner_review.md  ← Review sub-agent (creates beads, single-writer)
    ├── coder.md           ← Implementation agent
    └── reviewer.md        ← Code review agent

.codex/
├── config.toml            ← Codex multi-agent config
└── agents/
    ├── planner_draft.toml ← planner_draft sub-agent config
    └── planner_review.toml← planner_review sub-agent config
```
