---
schemaVersion: 1
surface: repo-authored-semantics
service: disasterrecovery
slug: drplan
gaps: []
---

# Logic Gaps

No open logic gaps remain for the seeded
`disasterrecovery/DrPlan` row after the reviewed runtime contract replaces the
scaffold placeholder with the published work-request-backed create and update
path plus resource-local mutable drift normalization.

## Current runtime contract

- `DrPlan` keeps the generated controller, service-manager shell, and
  registration wiring, but the live runtime contract is owned by
  `pkg/servicemanager/disasterrecovery/drplan/drplan_runtime_client.go`
  rather than the generated baseline in
  `pkg/servicemanager/disasterrecovery/drplan/drplan_serviceclient.go`.
- The vendored SDK exposes `Create/Get/List/Update/DeleteDrPlan` plus
  `GetWorkRequest`. `CreateDrPlan` returns a `DrPlan` body together with
  `opc-work-request-id`, `UpdateDrPlan` returns only work-request headers, and
  `DeleteDrPlan` returns no work-request header at all. `ListDrPlans` returns
  `DrPlanSummary` items rather than full `DrPlan` bodies.
- Pre-create lookup is explicit. `ListDrPlans` always scopes by exact
  `drProtectionGroupId` plus `displayName` and `type`, and tracked rereads can
  additionally narrow with `drPlanId` when a resource OCID is already known.
  Unique matches bind; duplicate matches fail instead of guessing.
- Create only writes the fields exposed by `CreateDrPlanDetails`:
  `displayName`, `type`, `drProtectionGroupId`, `sourcePlanId`,
  `freeformTags`, and `definedTags`. `spec.planGroups` is intentionally ignored
  during create because the OCI create contract does not accept plan group
  mutations up front.
- In-place mutation stays aligned with `UpdateDrPlanDetails` for
  `displayName`, `planGroups`, `freeformTags`, and `definedTags`.
  `drProtectionGroupId`, `sourcePlanId`, and `type` remain replacement-only
  drift.
- The resource-local update builder normalizes the live `GetDrPlan` body down
  to the `UpdateDrPlanDetails` surface before diffing, so service-owned fields
  such as `groupId`, `typeDisplayName`, `runOnInstanceRegion`,
  `refreshStatus`, and similar read-only step metadata do not cause perpetual
  updates. When `planGroups` are supplied, pause and step booleans are sent
  explicitly so `false` survives request projection.
- Create and update resume through `status.async.current` using
  `GetWorkRequest` plus `GetDrPlan` rereads. Delete confirmation is required
  but not work-request-backed: the runtime waits for `GetDrPlan` or
  `ListDrPlans` to prove that the tracked plan is gone or has reached lifecycle
  state `DELETED`.
- Required status projection stays truthful for `id`, `displayName`,
  `compartmentId`, `type`, `drProtectionGroupId`, `peerDrProtectionGroupId`,
  `peerRegion`, `planGroups`, `sourcePlanId`, `lifecycleState`,
  `lifecycleSubState`, `lifeCycleDetails`, and tag fields from concrete
  `GetDrPlan` rereads because list summaries omit `planGroups` and
  `sourcePlanId`.

## Authority and scoped cleanup

- `formal/controllers/disasterrecovery/drplan/*` is the authoritative formal
  path for the promoted `DrPlan` runtime contract.
- `RefreshDrPlan`, `VerifyDrPlan`, DR plan executions, and other disaster
  recovery helper families remain out of scope for this controller-backed
  rollout.
