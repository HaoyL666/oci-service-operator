---
schemaVersion: 1
surface: repo-authored-semantics
service: generativeaiagent
slug: agentendpoint
gaps: []
---

# Logic Gaps

No open logic gaps remain for the reviewed
`generativeaiagent/AgentEndpoint` contract. This story updates the
resource-local runtime semantics while the shared manifest row stays in the
scaffold stage until the convergence story handles the cross-resource
promotion.

## Current runtime path

- `AgentEndpoint` keeps the generated controller, service-manager shell, and
  registration wiring, but the live runtime contract is owned by
  `pkg/servicemanager/generativeaiagent/agentendpoint/agentendpoint_runtime_client.go`
  rather than the generated baseline in
  `pkg/servicemanager/generativeaiagent/agentendpoint/agentendpoint_serviceclient.go`.
- Create, update, and delete are work-request-backed. The runtime stores the
  in-flight OCI work request in `status.async.current`, normalizes
  Generative AI Agent `OperationStatus*` values (`ACCEPTED`, `IN_PROGRESS`,
  `WAITING`, `NEEDS_ATTENTION`, `SUCCEEDED`, `FAILED`, `CANCELING`, and
  `CANCELED`) into shared async classes, maps
  `CREATE_AGENT_ENDPOINT`, `UPDATE_AGENT_ENDPOINT`, and
  `DELETE_AGENT_ENDPOINT` into create/update/delete phases, and resumes
  reconciliation from that shared async tracker across requeues.
- Create-time identity recovery is work-request-backed. The runtime prefers the
  create response body when OCI returns an `AgentEndpoint` identifier and
  otherwise resolves the created OCID from work-request resources before
  rereading `GetAgentEndpoint`.
- Bind resolution is bounded. When no OCI identifier is tracked, the runtime
  only attempts pre-create reuse when `spec.displayName` is non-empty and then
  adopts only a unique `ListAgentEndpoints` match on exact `compartmentId`
  plus `agentId` plus `displayName` in reusable lifecycles (`ACTIVE`,
  `CREATING`, or `UPDATING`). Summaries in `FAILED`, `DELETING`, or `DELETED`
  are not reused, and duplicate exact-name matches fail instead of binding
  arbitrarily.
- Mutable drift is limited to `displayName`, `description`,
  `contentModerationConfig`, `guardrailConfig`, `metadata`,
  `humanInputConfig`, `outputConfig`, `shouldEnableTrace`,
  `shouldEnableCitation`, `shouldEnableMultiLanguage`, `sessionConfig`,
  `provisionedCapacityConfig`, `freeformTags`, and `definedTags`. The
  handwritten update-body builder preserves clear-to-empty intent for
  `description`, `metadata`, and tag maps, preserves false-valued booleans,
  treats nil readbacks for `contentModerationConfig` and `humanInputConfig`
  as false-equivalent zero values, and keeps
  `outputConfig.outputLocation` projected into a concrete SDK body.
  `compartmentId`, `agentId`, and `shouldEnableSession` stay create-only
  drift, and the provider-only `ChangeAgentEndpointCompartment` auxiliary
  operation stays out of scope for the published runtime.
- Reviewed lifecycle mapping treats `CREATING` as provisioning, `UPDATING` as
  updating, `ACTIVE` as success, `DELETING` as terminating, `DELETED` as the
  delete-confirmation target, and `FAILED` as terminal failure without
  requeue.
- Required status projection remains part of the repo-authored contract. The
  runtime projects OSOK lifecycle conditions, the shared async tracker, and
  the published read-model fields when OCI returns them, including `status.id`,
  `status.agentId`, `status.compartmentId`, `status.displayName`,
  `status.description`, `status.contentModerationConfig`,
  `status.guardrailConfig`, `status.metadata`, `status.humanInputConfig`,
  `status.outputConfig`, `status.shouldEnableTrace`,
  `status.shouldEnableCitation`, `status.shouldEnableSession`,
  `status.shouldEnableMultiLanguage`, `status.sessionConfig`,
  `status.provisionedCapacityConfig`, `status.timeCreated`,
  `status.timeUpdated`, `status.lifecycleState`, `status.lifecycleDetails`,
  `status.freeformTags`, `status.definedTags`, and `status.systemTags`.
- Delete confirmation is required, not best-effort. The finalizer stays until
  the delete work request is terminal and `GetAgentEndpoint` or fallback
  `ListAgentEndpoints` confirms the resource is gone or exposes lifecycle
  state `DELETED`.
