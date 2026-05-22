---
schemaVersion: 1
surface: repo-authored-semantics
service: databasemigration
slug: migration
gaps: []
---

# Logic Gaps

No open logic gaps remain for the seeded `databasemigration/Migration` row
after the runtime review replaced the scaffold placeholder with the published
work-request-backed generated-runtime contract.

## Current runtime path

- `Migration` keeps the generated controller, service-manager shell, and
  registration wiring, but the live runtime contract is owned by
  `pkg/servicemanager/databasemigration/migration/migration_runtime_client.go`
  rather than the generated baseline in
  `pkg/servicemanager/databasemigration/migration/migration_serviceclient.go`.
- The reviewed runtime requires explicit `spec.databaseCombination` and builds
  concrete `CreateMySqlMigrationDetails` or
  `CreateOracleMigrationDetails`, plus the matching concrete update body type,
  before OCI calls. The runtime rejects the unobservable object-helper surface
  (`spec.includeObjects`, `spec.excludeObjects`, and
  `spec.bulkIncludeExcludeData`) instead of claiming support for create-time
  inputs that `GetMigration` does not echo back after creation.
- Create-time and update-time type checks stay local to the resource package.
  `MYSQL` rejects Oracle-only fields such as `spec.advancedParameters`,
  `spec.sourceContainerDatabaseConnectionId`, and
  `spec.sourceStandbyDatabaseConnectionId`; `ORACLE` rejects the MySQL-only
  `initialLoadSettings` knobs that the published CRD surface exposes but the
  Oracle SDK shapes do not consume. The published runtime also limits
  `spec.dataTransferMediumDetails.type` to `OBJECT_STORAGE`, because the CRD
  does not expose the Oracle-only `DBLINK`, `NFS`, or `AWS_S3` detail shapes.
- Create, update, and delete are work-request-backed. The runtime stores the
  in-flight OCI work request in `status.async.current`, normalizes Database
  Migration `OperationStatus*` values (`ACCEPTED`, `IN_PROGRESS`, `WAITING`,
  `CANCELING`, `SUCCEEDED`, `FAILED`, and `CANCELED`) into shared async
  classes, maps `CREATE_MIGRATION`, `UPDATE_MIGRATION`, and
  `DELETE_MIGRATION` into create/update/delete phases, and resumes
  reconciliation from that shared async tracker across requeues.
- Create and update completion are both reread-backed. `CreateMigration`
  returns a `Migration` body plus `opc-work-request-id`, while
  `UpdateMigration` and `DeleteMigration` return work-request headers only, so
  the published runtime follows all three paths with `GetWorkRequest` and a
  concrete `GetMigration` reread before projecting status or confirming delete
  completion.
- Lifecycle handling is explicit: `ACCEPTED`, `CREATING`, `IN_PROGRESS`, and
  `WAITING` requeue as provisioning, `UPDATING` requeues as updating, `ACTIVE`,
  `INACTIVE`, and `SUCCEEDED` settle success, `DELETING` blocks finalizer
  release until `DELETED`, and any remaining terminal lifecycle such as
  `FAILED`, `CANCELED`, or `NEEDS_ATTENTION` surfaces as failure without
  requeue instead of being treated as ready.
- Required status projection remains part of the repo-authored contract. The
  runtime projects `status.id`, `status.displayName`,
  `status.compartmentId`, `status.type`,
  `status.sourceDatabaseConnectionId`,
  `status.targetDatabaseConnectionId`, `status.timeCreated`,
  `status.timeUpdated`, `status.timeLastMigration`,
  `status.lifecycleState`, `status.lifecycleDetails`,
  `status.waitAfter`, `status.executingJobId`,
  `status.assessmentId`, `status.databaseCombination`,
  `status.dataTransferMediumDetails`, `status.initialLoadSettings`,
  `status.advisorSettings`, `status.hubDetails`,
  `status.ggsDetails`, `status.sourceContainerDatabaseConnectionId`,
  `status.sourceStandbyDatabaseConnectionId`,
  `status.advancedParameters`, `status.freeformTags`,
  `status.definedTags`, and `status.systemTags` when OCI returns them.
  The published status intentionally excludes the hub admin password even
  though the spec accepts it, because `GetMigration` redacts that field from
  the live read model.

## Repo-authored semantics

- Pre-create reuse is exact-match and opt-in. The runtime only attempts
  bind-before-create when `compartmentId`, `displayName`,
  `databaseCombination`, `type`, `sourceDatabaseConnectionId`, and
  `targetDatabaseConnectionId` are all present. It lists by compartment and
  display name, narrows summaries in memory by the same stable identity
  fields, then rereads each candidate through `GetMigration` before binding.
- Mutation policy is explicit: only the `UpdateMigrationDetails` surface
  reconciles in place. `compartmentId`, `assessmentId`, and
  `databaseCombination` remain replacement-only drift, while the out-of-scope
  object-helper fields are rejected before OCI mutation rather than accepted
  as invisible or non-convergent config.
- Hub admin password handling is explicit: the runtime allows
  `hubDetails.restAdminCredentials.password` on create and includes it in an
  update request when another hub mutation already requires an update, but it
  does not treat password-only drift as parity because `GetMigration` does not
  expose that secret after the resource is created.
