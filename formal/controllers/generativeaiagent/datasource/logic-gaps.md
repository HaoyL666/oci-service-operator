---
schemaVersion: 1
surface: repo-authored-semantics
service: generativeaiagent
slug: datasource
gaps: []
---

# Logic Gaps

No controller-local logic gaps remain for the reviewed
`generativeaiagent/DataSource` runtime. Shared catalog promotion for this row is
intentionally deferred to the convergence story, so the manifest stays
scaffold-staged even though the resource-local runtime contract below is now
explicit.

## Current runtime path

- `DataSource` keeps the generated controller, service-manager shell, and
  registration wiring, but the live runtime contract is owned by
  `pkg/servicemanager/generativeaiagent/datasource/datasource_runtime_client.go`
  rather than the generated baseline in
  `pkg/servicemanager/generativeaiagent/datasource/datasource_serviceclient.go`.
- Create, update, and delete are work-request-backed. The runtime stores the
  in-flight OCI work request in `status.async.current`, normalizes Generative
  AI Agent `OperationStatus*` values (`ACCEPTED`, `IN_PROGRESS`, `WAITING`,
  `NEEDS_ATTENTION`, `SUCCEEDED`, `FAILED`, `CANCELING`, and `CANCELED`) into
  shared async classes, maps `CREATE_DATA_SOURCE`, `UPDATE_DATA_SOURCE`, and
  `DELETE_DATA_SOURCE` into create/update/delete phases, and resumes
  reconciliation from that shared async tracker across requeues.
- Create-time identity recovery is work-request-backed. The runtime prefers the
  create response body when OCI returns a `DataSource` identifier and otherwise
  resolves the created resource OCID from work-request resources before reading
  `GetDataSource` by ID and projecting status.
- Bind resolution is bounded. When no OCI identifier is tracked, the runtime
  only attempts pre-create reuse when `spec.displayName` is non-empty and then
  adopts only a unique `ListDataSources` match on exact `compartmentId` plus
  `knowledgeBaseId` plus `displayName` in reusable lifecycles (`ACTIVE`,
  `INACTIVE`, `CREATING`, or `UPDATING`). Summaries in `FAILED`, `DELETING`, or
  `DELETED` are not reused, and duplicate exact-name matches fail instead of
  binding arbitrarily.
- Mutable drift is limited to `displayName`, `description`, `dataSourceConfig`,
  `metadata`, `freeformTags`, and `definedTags`. The handwritten create/update
  builders rebuild `dataSourceConfig` into concrete SDK polymorphic bodies,
  preserve clear-to-empty intent for `description`, `metadata`, and tag maps,
  and support raw `dataSourceConfig.jsonData` when it resolves to the pinned
  `OCI_OBJECT_STORAGE` SDK type. `compartmentId` and `knowledgeBaseId` remain
  replacement-only drift.
- Reviewed lifecycle mapping treats `CREATING` as provisioning, `UPDATING` as
  updating, `ACTIVE` and `INACTIVE` as success, `DELETING` as terminating,
  `DELETED` as the delete-confirmation target, and `FAILED` as terminal failure
  without requeue.
- Required status projection remains part of the repo-authored contract. The
  runtime projects OSOK lifecycle conditions, the shared async tracker, and the
  published read-model fields when OCI returns them, including `status.id`,
  `status.displayName`, `status.description`, `status.knowledgeBaseId`,
  `status.dataSourceConfig`, `status.timeCreated`, `status.timeUpdated`,
  `status.lifecycleState`, `status.lifecycleDetails`, `status.compartmentId`,
  `status.metadata`, `status.freeformTags`, `status.definedTags`, and
  `status.systemTags`.
- Delete confirmation is required, not best-effort. The finalizer stays until
  the delete work request is terminal and `GetDataSource` or fallback
  `ListDataSources` confirms the resource is gone or exposes lifecycle state
  `DELETED`.
