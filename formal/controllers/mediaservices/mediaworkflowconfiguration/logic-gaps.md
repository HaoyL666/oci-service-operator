---
schemaVersion: 1
surface: repo-authored-semantics
service: mediaservices
slug: mediaworkflowconfiguration
gaps: []
---

# Logic Gaps

This row stays in `scaffold` stage until the convergence story updates the
manifest-owned promotion metadata, but the generated runtime placeholder is no
longer the published contract.

## Current runtime contract

- `MediaWorkflowConfiguration` keeps the generated controller,
  service-manager shell, and registration wiring, but the reviewed runtime
  contract is owned by
  `pkg/servicemanager/mediaservices/mediaworkflowconfiguration/mediaworkflowconfiguration_runtime_client.go`
  instead of the generated baseline in
  `pkg/servicemanager/mediaservices/mediaworkflowconfiguration/mediaworkflowconfiguration_serviceclient.go`.
- Create, get, and update return the `MediaWorkflowConfiguration` body
  directly, while delete returns only headers and no service-local
  work-request identifier. The reviewed runtime therefore treats create and
  update as synchronous read-after-write flows, settles `ACTIVE` as success,
  and confirms delete by follow-up `GetMediaWorkflowConfiguration` or fallback
  `ListMediaWorkflowConfigurations` rereads. A post-delete reread that still
  returns `ACTIVE` is treated as delete pending and keeps the finalizer until
  OCI eventually reports `DELETED` or NotFound.
- Pre-create lookup is explicit and conservative. The runtime requires
  `spec.compartmentId`, skips reuse when `spec.displayName` is empty, scopes
  `ListMediaWorkflowConfigurations` by exact compartment and display name, and
  adopts only a unique exact match on the reviewed identity surface
  `compartmentId + displayName`. Duplicate matches fail instead of binding
  arbitrarily.
- Mutable drift is limited to `displayName`, `parameters`, `freeformTags`, and
  `definedTags`. The handwritten update-body builder preserves explicit
  clear-to-empty intent for `parameters` and tag maps. `compartmentId` remains
  replacement-only drift, and the runtime does not publish lock overrides on
  update or delete.
- Matching create-time locks are normalized out of steady-state parity by
  ignoring OCI-populated `timeCreated` metadata. Changing the requested locks
  after create, including omitting `spec.locks` while OCI still reports
  create-time locks, remains explicit create-only drift rather than an
  implicit add/remove lock flow.
- Required status projection remains part of the repo-authored contract. The
  runtime projects identifiers, timestamps, lifecycle state, lifecycle
  details, parameter values, tag maps, locks, and the shared OSOK condition
  read model from observed OCI responses when those fields are available.
- The row remains scaffold-owned only because shared manifest/index promotion is
  deferred. The runtime contract above is the controller-backed behavior that
  reviewers should validate for this story.
