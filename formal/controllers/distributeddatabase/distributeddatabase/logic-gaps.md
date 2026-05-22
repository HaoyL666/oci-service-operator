---
schemaVersion: 1
surface: repo-authored-semantics
service: distributeddatabase
slug: distributeddatabase
gaps: []
---

# Logic Gaps

The shared manifest row stays `scaffold` until `S10`, but the
controller-local runtime contract for
`distributeddatabase/DistributedDatabase` is reviewed on this branch and no
additional resource-local logic gaps remain.

## Current runtime contract

- `DistributedDatabase` keeps the generated controller, service-manager shell,
  and registration wiring, but the published runtime uses generatedruntime plus
  a package-local overlay in
  `pkg/servicemanager/distributeddatabase/distributeddatabase/` for reviewed
  lifecycle semantics, polymorphic create-body normalization, explicit
  update-body fidelity, and tracked-identity cleanup.
- CRUD is lifecycle-driven. The runtime treats `CREATING` as provisioning,
  `UPDATING` as updating, `ACTIVE` and `INACTIVE` as success,
  `NEEDS_ATTENTION` and `FAILED` as terminal non-success, and confirms delete
  through `DELETING` until `DELETED` or NotFound.
- Pre-create lookup is explicit. The runtime lists by exact `compartmentId`
  plus `displayName`, preserves the service-supported `dbDeploymentType` and
  `lifecycleState` filters when they are already available for rereads, then
  narrows summary candidates in memory to a unique exact `prefix` plus
  `dbDeploymentType` match before reuse.
- Create-body normalization is explicit. The package-local builder strips the
  scaffold-only `jsonData` field, preserves explicit `false` booleans in nested
  `vmClusterDetails` and backup-config payloads, and normalizes CRD
  `EXISTING_CLUSTER` inputs to the OCI SDK's `EXADB_XS` create-family types for
  shard and catalog bodies.
- Update semantics are intentionally narrow on the published CRD surface. The
  runtime sends only `displayName`, `freeformTags`, and `definedTags` through
  `UpdateDistributedDatabaseDetails`; `compartmentId` remains replacement-only
  drift because the current CRD does not publish the separate
  change-compartment helper, and the provider-only patch, state-transition, and
  trigger families remain out of scope on this row.
- `ListDistributedDatabases` returns `DistributedDatabaseSummary`, which omits
  fields such as `scanListenerPort`, `latestGsmImageDetails`, `shardDetails`,
  `catalogDetails`, `gsmDetails`, `dbBackupConfig`, and `gsmSshPublicKey`. The
  runtime therefore forces a follow-up `GetDistributedDatabase` after bind,
  create, and update so status projection stays truthful. The row has no
  Kubernetes Secret side effects.
