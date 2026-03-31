---
schemaVersion: 1
surface: repo-authored-semantics
service: mysql
slug: mysqldbsystem
gaps:
  - category: bind-versus-create
    status: open
    stopCondition: "Formal semantics can branch between create, bind-by-id, and bind-by-display-name flows without routing through DbSystemServiceManager."
  - category: list-lookup
    status: open
    stopCondition: "Formal semantics encode the current displayName plus compartment lookup and the ACTIVE, CREATING, UPDATING, or INACTIVE lifecycle filter used before create or bind."
  - category: secret-input
    status: open
    stopCondition: "Formal semantics model the required username and password secret reads before create."
  - category: endpoint-materialization
    status: open
    stopCondition: "Formal semantics model the ACTIVE-only secret write that publishes IP address, FQDN, ports, and endpoint JSON."
  - category: status-projection
    status: open
    stopCondition: "Formal semantics either describe the handwritten OsokStatus projection or preserve it as an explicit legacy-only contract."
  - category: mutation-policy
    status: open
    stopCondition: "Formal semantics enumerate the limited mutable fields and keep immutable inputs from silently drifting into update requests."
  - category: delete-confirmation
    status: open
    stopCondition: "Delete is represented as an explicit unsupported path or replaced with a safe OCI delete plus confirmation flow before promotion."
  - category: legacy-adapter
    status: resolved
    stopCondition: "mysqldbsystem_generated_runtime_client.go preserves the legacy DbSystemServiceManager secret, lookup, mutation, and delete behavior, so MySqlDbSystem no longer needs mysqldbsystem_generated_client_adapter.go."
---

# Logic Gaps

## Current runtime path

- `MySqlDbSystem` now routes through the custom generated-runtime client in
  `pkg/servicemanager/mysql/dbsystem/mysqldbsystem_generated_runtime_client.go`;
  the repo no longer keeps a full `mysqldbsystem_generated_client_adapter.go`
  delegate to `DbSystemServiceManager`.
- The handwritten runtime hook still checks `spec.id` first; when no OCI ID is
  tracked, it lists DB systems by `compartmentId` and `displayName` and only
  binds to results in `ACTIVE`, `CREATING`, `UPDATING`, or `INACTIVE` before
  falling back to create.
- Create, update, and delete still preserve repo-authored mysql behavior, but
  they now do it through generatedruntime callbacks instead of the standalone
  legacy manager.

## Shared baseline alignment

- Use the [shared generated-runtime baseline](../../../shared/generated-runtime-baseline.md)
  as the category map for bind, lookup, waiter, mutation, status, secret, and
  delete decisions.
- `mysql/MySqlDbSystem` keeps the same category names as the shared baseline,
  but now expresses them through mysql-specific generatedruntime callbacks
  instead of a full adapter handoff.
- `streaming/Stream` remains the reference for naming bind, lookup, secret,
  and delete categories, while `identity/User` remains the clean
  formal-promotion precedent that mysql has not reached yet.

## Repo-authored semantics

- Create requires two Kubernetes secrets: `spec.adminUsername` must expose a `username` key and `spec.adminPassword` must expose a `password` key.
- After the DB system reaches ACTIVE, the manager writes a Kubernetes secret containing `PrivateIPAddress`, `InternalFQDN`, `AvailabilityDomain`, `FaultDomain`, `MySQLPort`, `MySQLXProtocolPort`, and serialized endpoint data.
- Status projection is manual. Only `OsokStatus` is recorded even though the OCI response carries richer state.
- Update is intentionally narrow: the handwritten logic only mutates display name, description, configuration ID, and tags, even though imported provider facts expose a much broader mutable set.
- Delete currently returns success without issuing an OCI delete request, so finalizer removal is not confirmation.

## Why formal promotion still stays blocked

- Promotion remains blocked by `oci-service-operator-8xa.8` until the
  remaining mysql-only secret, mutation, and delete semantics move into the
  formal model without another handwritten shim.
- Secret reads and endpoint secret materialization are still OSOK-only
  semantics layered on top of generatedruntime; they do not come from provider
  facts alone.
- The bind-versus-create, mutation, and delete rules remain more specific than
  the generic runtime defaults, so promotion must still wait until the formal
  model can express them directly even though the full adapter shim is gone.
