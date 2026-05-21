---
schemaVersion: 1
surface: repo-authored-semantics
service: jms
slug: jmsplugin
gaps: []
---

# Logic Gaps

This story replaces the placeholder JmsPlugin runtime notes under the still
`scaffold` `jms/JmsPlugin` manifest row. Shared manifest and index promotion
remain deferred to the convergence story, but the local controller files now
record the reviewed direct-body generatedruntime contract for this resource.

## Current runtime path

- `JmsPlugin` keeps the generated controller, service-manager shell, and
  registration wiring, but the live runtime contract is owned by
  `pkg/servicemanager/jms/jmsplugin/jmsplugin_runtime_client.go` rather than
  the generated baseline in `pkg/servicemanager/jms/jmsplugin/jmsplugin_serviceclient.go`.
- The vendored JMS SDK exposes direct `CreateJmsPlugin`, `GetJmsPlugin`,
  `ListJmsPlugins`, `UpdateJmsPlugin`, and `DeleteJmsPlugin` operations.
  Create and update return a `JmsPlugin` body immediately, while delete
  returns headers only and relies on rereads for confirmation. The reviewed
  runtime therefore publishes no provisioning or updating lifecycle buckets
  and keeps `async.strategy=none`.
- Reviewed lifecycle classification is explicit: `ACTIVE` and `INACTIVE`
  settle success, `DELETED` is only a delete-confirmation target, and
  `NEEDS_ATTENTION` stays outside the published success set instead of being
  rebound or treated as a steady state.
- Bind resolution is bounded and paginated. Existing-before-create lookup
  requires `spec.compartmentId` plus `spec.agentId`, exhausts paginated
  `ListJmsPlugins` responses, and only adopts a unique exact
  `compartmentId+agentId` match from an `ACTIVE` or `INACTIVE` summary.
  Duplicate exact matches fail instead of binding arbitrarily.
- Mutable drift is limited to `fleetId`, `freeformTags`, and `definedTags`.
  The handwritten update-body builder preserves empty-map clears for both tag
  surfaces, performs in-place fleet moves when a non-empty desired fleet OCID
  differs from OCI, and leaves fleet detachment out of scope because the
  published string spec cannot distinguish omission from clear. `agentId`,
  `agentType`, and `compartmentId` remain replacement-only drift.
- Delete confirmation is required, not best-effort. The finalizer stays until
  `GetJmsPlugin` or fallback `ListJmsPlugins` confirms the plugin is gone or
  reports lifecycle state `DELETED`.
- Required status projection remains part of the repo-authored contract. The
  runtime keeps the shared OSOK status plus the published
  `status.id`, `status.agentId`, `status.agentType`, `status.lifecycleState`,
  `status.availabilityStatus`, `status.timeRegistered`, `status.fleetId`,
  `status.compartmentId`, `status.hostname`, `status.osFamily`,
  `status.osArchitecture`, `status.osDistribution`, `status.pluginVersion`,
  `status.timeLastSeen`, `status.freeformTags`, `status.definedTags`, and
  `status.systemTags` read-model fields when OCI returns them.
