# Codemap — OCI Service Operator

Quick-reference for agents. Read this instead of exploring the repo.

## Architecture

```
main.go                                  ← Controller registration (3 reconcilers)
├── api/<service>/v1beta1/*_types.go     ← CRD Spec/Status definitions (grouped by OCI service)
├── controllers/<service>/*_controller.go ← Reconcile loops (grouped by OCI service)
├── pkg/core/reconciler.go               ← BaseReconciler — shared reconciliation logic
├── pkg/servicemanager/*/                ← OCI API client wrappers per service
│   ├── *_serviceclient.go               ← OCI SDK interface + client creation
│   ├── *_servicemanager.go              ← CreateOrUpdate / Delete / GetCrdStatus
│   └── *_secretgeneration.go            ← Secret payload after ACTIVE state
├── pkg/validator/                       ← API spec + SDK parity validator
├── pkg/config/                          ← OSOK + user auth configuration
├── pkg/credhelper/                      ← Kubernetes secret-based credential helper
├── pkg/authhelper/                      ← OCI auth config provider
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
├── database/autonomousdatabases_controller.go
├── streaming/stream_controller.go
├── mysql/mysqldbsystem_controller.go
└── suite_test.go
```

main.go imports them as namespaced packages:
```go
databasecontrollers "...controllers/database"
streamingcontrollers "...controllers/streaming"
mysqlcontrollers "...controllers/mysql"
```

## Implemented Services (3 with controllers)

| Service | Types | Controller | Service Manager | Docs |
|---------|-------|-----------|----------------|------|
| Autonomous DB | `api/database/v1beta1/` | `controllers/database/` | `autonomousdatabases/adb/` (5 files) | `adb.md` |
| MySQL | `api/mysql/v1beta1/` | `controllers/mysql/` | `mysql/dbsystem/` (5 files) | `mysql.md` |
| Streams | `api/streaming/v1beta1/` | `controllers/streaming/` | `streams/` (4 files) | `oss.md` |

## Service Manager Files

```
pkg/servicemanager/
├── interfaces.go                                    ← OSOKServiceManager interface
├── autonomousdatabases/adb/
│   ├── adb_serviceclient.go                         ← OCI SDK interface
│   ├── adb_servicemanager.go                        ← Business logic
│   ├── adb_servicemanager_test.go                   ← Tests
│   ├── adb_walletclient.go                          ← Wallet download client
│   └── export_test.go                               ← Test helpers
├── mysql/dbsystem/
│   ├── dbsystem_serviceclient.go                    ← OCI SDK interface
│   ├── dbsystem_servicemanager.go                   ← Business logic
│   ├── dbsystem_servicemanager_test.go              ← Tests
│   ├── dbsystem_secretgeneration.go                 ← Secret payload
│   └── export_test.go                               ← Test helpers
└── streams/
    ├── stream_serviceclient.go                      ← OCI SDK interface
    ├── stream_servicemanager.go                     ← Business logic
    ├── stream_servicemanager_test.go                ← Tests
    └── stream_secretgeneration.go                   ← Secret payload
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
| Simple service manager | `pkg/servicemanager/streams/` | 3-file pattern, easy to follow |
| Controller | `controllers/streaming/stream_controller.go` | Clean BaseReconciler delegation |
| Secret generation | `pkg/servicemanager/mysql/dbsystem/dbsystem_secretgeneration.go` | GetCredentialMap pattern |
| Tests | `pkg/servicemanager/autonomousdatabases/adb/adb_servicemanager_test.go` | Injected client, error paths |
| RBAC roles | `config/rbac/stream_editor_role.yaml` | Minimal role template |
| Sample manifest | `config/samples/oci_v1beta1_stream.yaml` | Simple sample |
| Docs | `docs/oss.md` | Simple service documentation |

## File Naming Conventions

```
api/<service>/v1beta1/<resource>_types.go             ← CRD types (grouped by service)
controllers/<service>/<resource>_controller.go        ← Controller (grouped by service)
pkg/servicemanager/<svc>/<svc>_serviceclient.go       ← OCI client interface
pkg/servicemanager/<svc>/<svc>_servicemanager.go      ← Business logic
pkg/servicemanager/<svc>/<svc>_secretgeneration.go    ← Secret payload
pkg/servicemanager/<svc>/<svc>_servicemanager_test.go ← Tests
pkg/servicemanager/<svc>/export_test.go               ← Test helpers
config/samples/oci_v1beta1_<resource>.yaml            ← Sample manifest
config/rbac/<resource>_editor_role.yaml               ← RBAC
docs/<service>.md                                     ← Documentation
```

## Key Patterns (MUST follow)

1. **OCI client interface injection** — Every service manager has `ociClient <Interface>` field, nil = create from Provider
2. **Lifecycle states** — FAILED → set failed status; ACTIVE → set active; other → requeue
3. **Conditional fields** — `if spec.X != "" { details.X = common.String(spec.X) }` — never send zero-values
4. **Secret generation** — `GetCredentialMap()` returns `map[string]string` after ACTIVE
5. **Registration** — New controllers registered in `main.go` via `SetupWithManager` (use namespaced import)
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
