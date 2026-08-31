# Controller Integration Testing

OSOK uses complementary deterministic and live tests for generated and
handwritten service-manager behavior.

## Test Layers

`make test` remains the repository build, generation, envtest, and unit-test
gate. `make integrationtest` adds resource reconciliation through the real OCI
SDK HTTP surface using checked-in sanitized cassettes. `make e2e-live` installs
a service controller in Kind and performs a real OCI create, update, and delete
lifecycle.

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

Record mode delegates to the real SDK HTTP client and writes the cassette
atomically. Replay mode does not open a network connection. It matches unused
interactions by method, host, path, canonical query, selected semantic headers,
and canonical request body. Interaction order may vary across concurrent
reconciles, but duplicate requests consume their recorded responses in order.

Before a cassette is written, the recorder:

- removes authorization and volatile signing headers;
- redacts retry tokens and request identifiers;
- replaces OCI identifiers with stable placeholders;
- redacts JSON keys containing credentials, tokens, passwords, private keys,
  fingerprints, or secrets;
- rejects private-key and security-token markers;
- limits captured request and response sizes.

Replay restores recorded OCI identifiers to the test's input values and creates
stable synthetic OCIDs for resource identifiers first returned by OCI. This
keeps spec-to-status comparisons meaningful.

The Budget integration test is the initial reference:

```text
pkg/servicemanager/budget/budget/budget_recorded_integration_test.go
pkg/servicemanager/budget/budget/testdata/recordings/budget_crud.yaml
```

A new cassette test should cover create, read-after-create, update,
read-after-update, delete, and confirmed not-found whenever the OCI resource
supports those operations. Tests for immutable resources can omit update and
state that in the test name.

Do not commit raw recordings. Review the sanitized cassette before adding it to
the repository.

## Live Lifecycle Scenarios

Each scenario owns a `scenario.yaml`, a create manifest, an optional update
manifest, and optional dependency manifests. All referenced files must remain
inside the scenario directory; absolute paths, traversal, and escaping symlinks
are rejected.

The runner performs:

1. dependency apply in declared order;
2. primary resource create;
3. readiness and status convergence;
4. optional update and renewed convergence;
5. primary resource deletion and not-found confirmation;
6. dependency deletion in reverse order.

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
profile, while resource-specific values such as `OCI_COMPARTMENT_ID` remain
explicit operator inputs.

## Coverage Expectations

Start with one safe representative resource per runtime family rather than one
expensive test per generated type. Add targeted scenarios for handwritten
runtime behavior, work-request lifecycles, ForceNew handling, and secret output.
When a service or SDK change affects a resource without replay or live coverage,
report that gap explicitly during review.
