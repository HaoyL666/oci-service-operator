# Controller Integration Testing

OSOK uses complementary deterministic and live tests for generated and
handwritten service-manager behavior.

## Test Layers

`make test` remains the repository build, generation, envtest, and unit-test
gate. `make replaytest` exercises the real OCI SDK HTTP surface using checked-in
sanitized cassettes without credentials or network access. `make
integrationtest` combines those replays with the deterministic lifecycle-runner
tests. `make e2e-live` installs a service controller in Kind and performs a real
OCI create, update, and delete lifecycle.

The cassette suite follows the Azure Service Operator record/replay pattern:
the same SDK requests and responses can be replayed without credentials, quota,
or cloud availability. The live scenario follows the AWS ACK pattern: it waits
for the Kubernetes resource to reconcile, verifies observed state, mutates the
resource, and confirms deletion from the controller's perspective.

Neither test replaces the other. Replay detects request, response, mapping, and
reconciliation regressions. Live CRUD detects OCI behavior that an older
recording cannot represent.

## OCI Cassettes

The `internal/e2e/ocireplay` package implements the OCI SDK HTTP dispatcher
interface. A resource integration test attaches a cassette to the SDK client's
embedded `common.BaseClient`, then constructs its normal runtime hooks and
service client.

Every new cassette declares metadata:

```yaml
metadata:
  service: budget
  resource: Budget
  operations: [create, read, update, delete]
  sdkVersion: v65.110.0
  provenance: recorded
```

`provenance` is either `recorded` for interactions captured from a real OCI API
or `synthetic` for interactions authored from the checked-in OCI SDK/API
contract. Synthetic cassettes must not be described as live OCI evidence.
Operation names are limited to `create`, `read`, `update`, `delete`, and
`action`.

Record mode delegates to the real SDK HTTP client and writes the cassette
atomically. Replay mode does not open a network connection. It matches unused
interactions by method, host, path, canonical query, selected semantic headers,
and canonical request body. Interaction order may vary across concurrent
reconciles, but duplicate requests consume their recorded responses in order.

Recorded integration tests default to replay. Synthetic integration tests are
always replay-only and use the `*_synthetic_integration_test.go` naming suffix
so they cannot be mistaken for live OCI evidence. An operator must explicitly opt
into live OCI traffic:

```bash
OSOK_OCI_CASSETTE_MODE=record \
OSOK_OCI_RECORD_AUTH=security_token \
OCI_CONFIG_PROFILE=YOUR_PROFILE \
OCI_COMPARTMENT_ID=ocid1.compartment.oc1..replace_me \
go test ./pkg/servicemanager/SERVICE/RESOURCE \
  -run '^TestRecordedRESOURCECreateUpdateDelete$' -count=1 -v
```

Use `OSOK_OCI_CASSETTE_OVERWRITE=true` only when intentionally replacing an
existing reviewed cassette. `OSOK_OCI_RECORD_AUTH` accepts `security_token` or
`user_principal`; configuration comes from `OCI_CONFIG_FILE` and
`OCI_CONFIG_PROFILE`. A recording test must use test-owned names, clean up its
OCI resource, and keep external prerequisite OCIDs in environment variables.
The test publishes its cassette only after the full lifecycle succeeds; a
failed recording cleans up OCI state without replacing the previous cassette.

Before a cassette is written, the recorder:

- removes authorization and volatile signing headers;
- redacts retry tokens and request identifiers;
- replaces OCI identifiers with stable placeholders;
- replaces explicitly bound non-OCID values, such as an Object Storage
  namespace, with named placeholders;
- redacts JSON keys containing credentials, tokens, passwords, private keys,
  fingerprints, creator identities, or secrets;
- rejects private-key and security-token markers;
- limits captured request and response sizes.

Placeholder assignment traverses query and JSON keys in canonical order, so a
request containing several OCIDs replays deterministically across processes.
Avoid recording broad list operations when they could capture unrelated tenant
resources; use the runtime's bounded skip-existing-before-create context for a
test-owned unique resource instead.

Replay restores recorded OCI identifiers to the test's input values and creates
stable synthetic OCIDs for resource identifiers first returned by OCI. This
keeps spec-to-status comparisons meaningful.

Use `ocireplay.OpenSDKReplay` to construct a credential-free, no-retry OCI SDK
base client and verify the cassette metadata before the first interaction. The
session's `Close` method fails when any recorded interaction was not consumed.
The lower-level `Open` API remains available to record and test the dispatcher
itself.

The Budget integration test is the initial reference:

```text
pkg/servicemanager/budget/budget/budget_recorded_integration_test.go
pkg/servicemanager/budget/budget/testdata/recordings/budget_crud.yaml
```

A new cassette test should cover create, read-after-create, update,
read-after-update, delete, and confirmed not-found whenever the OCI resource
supports those operations. Tests for immutable resources can omit update and
state that in the test name.

Synthetic cassettes use the checked-in OCI SDK models and service contract when
live creation is impractical because of permissions, quota, cost, regional
availability, or external prerequisites. Name their tests `TestSynthetic...`,
set `provenance: synthetic`, and do not provide record-mode behavior. They prove
request construction and response handling, not live OCI behavior.

The initial synthetic matrix is intentionally limited to resources with clear
external side effects or prerequisites:

- Access Governance instances require a service entitlement and an IDCS
  administrator token.
- Rover clusters represent orders for physical appliances.
- Generative AI dedicated AI clusters consume scarce paid accelerator
  capacity.
- MySQL DB Systems provision billable compute and block storage and have long
  create/delete lifecycles.
- OCI VMware Solution SDDCs allocate multiple billable bare-metal ESXi hosts
  and require a purpose-built subnet and VLAN topology.
- OpenSearch clusters allocate several compute nodes plus block storage and
  require VCN/subnet infrastructure.

Do not replace these with live recordings unless a service owner provides an
isolated entitlement and explicitly approves the external side effects.

Synthetic cassettes also cover failure paths that should not be induced against
live OCI merely for testing. The initial failure matrix proves:

- an Object Storage create can surface throttling and succeed on a later
  reconciliation;
- an Object Storage delete completes when a confirmation read proves the bucket
  disappeared before the delete request;
- a failed Queue work request becomes a terminal failed status with preserved
  asynchronous evidence.

The expanded recorded matrix also covers commonly used, bounded OCI resources:

- Notifications Topic;
- disabled Monitoring Alarm with a temporary Topic destination;
- single-partition Streaming Stream in the default pool;
- minimally provisioned NoSQL Table.
- Core Route Table with normalized empty rules;
- Core Security List with a mutable egress rule;
- Core NAT Gateway with generated public-IP intent.
- Core Service Gateway with a regional Oracle Services Network attachment.
- empty private Artifacts Container Repository with eventually-consistent ID recovery.
- DevOps Project with generated work-request create, update, and delete handling.
- private Network Load Balancer with succeeded-work-request deletion evidence.
- Functions Application with subnet-backed create, configuration/tag update,
  and confirmed deletion.
- Logging Log Group with mutable description/tag verification and confirmed
  deletion.
- private DNS Zone in a temporary VCN resolver view, including acknowledged
  delete confirmation for the service's auth-shaped 404 response.

Each live recorder owns fixed test naming, update assertions, best-effort
failure cleanup, and confirmed terminal deletion before publishing its
cassette. Service-specific limits remain part of the recorder: NoSQL uses a
65-second DDL polling interval so retries do not perpetuate its per-minute DDL
limit.

Do not commit raw recordings. Review the sanitized cassette before adding it to
the repository.

Run only the SDK HTTP replay layer with:

```bash
make replaytest
```

Inspect current controller-to-cassette coverage with:

```bash
make replay-coverage
```

The coverage audit derives its inventory from checked-in controller files and
correlates them with cassette metadata and the recorded integration test that
references each cassette. During the phased rollout it reports missing,
legacy, unreferenced, and orphan cassettes without failing. Coverage
enforcement is intentionally deferred until the controller-backed surface has
been classified.

## Live Lifecycle Scenarios

Each scenario owns a `scenario.yaml`, a create manifest, an optional update
manifest, and optional dependency manifests. All referenced files must remain
inside the scenario directory; absolute paths, traversal, and escaping symlinks
are rejected.

The runner performs:

1. dependency apply in declared order;
2. primary resource create;
3. readiness, status convergence, and optional related-object assertions;
4. optional update and renewed convergence/assertions;
5. primary resource deletion, not-found confirmation, and optional related-object deletion;
6. dependency deletion in reverse order.

The local bootstrap also installs the pinned cert-manager release when a
selected package contains `Certificate` or `Issuer` resources. Packages without
cert-manager resources do not pay that setup cost.

On failure it makes a best-effort cleanup and preserves the original failure
separately from any cleanup error. Rendered manifests and `result.json` are
written below the ignored E2E artifact directory.

Run the Object Storage reference scenario with:

```bash
export OCI_COMPARTMENT_ID=ocid1.compartment.oc1..replace_me
make e2e-live
```

For another service:

```bash
export E2E_SERVICE=streaming
export E2E_SCENARIO=e2e/scenarios/streaming/basic/scenario.yaml
make e2e-live
```

Only use dependencies created for the scenario when
`cleanupDependencies: true`. Never place credentials or private keys in a
checked-in manifest; inject required values through environment placeholders.
The wrapper derives `OCI_TENANCY_ID` and `OCI_REGION` from the selected OCI
profile and, when OCI CLI access is available, discovers the first
`OCI_AVAILABILITY_DOMAIN` visible from the target compartment. Resource-specific
values such as `OCI_COMPARTMENT_ID` remain explicit operator inputs.

## Coverage Expectations

Start with one safe representative resource per runtime family rather than one
expensive test per generated type. Add targeted scenarios for handwritten
runtime behavior, work-request lifecycles, ForceNew handling, and secret output.
When a service or SDK change affects a resource without replay or live coverage,
report that gap explicitly during review.
