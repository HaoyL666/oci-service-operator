---
schemaVersion: 1
surface: repo-authored-semantics
service: tenantmanagercontrolplane
slug: domaingovernance
gaps: []
---

# Logic Gaps

## Current runtime path

- `DomainGovernance` keeps the generated controller, service-manager shell, and
  registration wiring, but the reviewed runtime contract is finalized in
  `pkg/servicemanager/tenantmanagercontrolplane/domaingovernance/domaingovernance_runtime_client.go`.
- Create and update are direct-body OCI mutations. `CreateDomainGovernance` and
  `UpdateDomainGovernance` return concrete `DomainGovernance` bodies, so the
  runtime projects status from the returned body directly, treats both
  `ACTIVE` and `INACTIVE` as settled success, and does not use service-local
  work-request polling.
- Bind resolution is exact and paginated. When no OCI identifier is tracked,
  the runtime paginates `ListDomainGovernances`, scopes the request by
  `compartmentId` and `domainId`, and adopts only a unique exact match on
  `domainId + onsTopicId + onsSubscriptionId`. Duplicate matches fail instead
  of binding arbitrarily.
- Mutable drift is limited to `subscriptionEmail`, `isGovernanceEnabled`,
  `freeformTags`, and `definedTags`. `compartmentId`, `domainId`,
  `onsTopicId`, and `onsSubscriptionId` remain replacement-only because the
  visible update request does not expose truthful in-place mutation for them.
- Delete is header-only and confirm-delete-driven. `DeleteDomainGovernance`
  returns no body and no work-request identifier, so the runtime treats any
  still-readable `DomainGovernance` after delete as terminating and keeps the
  finalizer until `GetDomainGovernance` or `ListDomainGovernances` return not
  found.
- Required status projection publishes the bound governance identity, owner and
  domain OCIDs, lifecycle, ONS identifiers, governance flag, notification
  email, timestamps, and tags with no secret side effects.
