# Service-Manager Mock Integration

Mock integration tests exercise a typed OSOK custom resource through its
production service manager and the real OCI Go SDK while replacing only the
SDK HTTP dispatcher. They are faster and broader than live OCI E2E, and deeper
than package tests that replace the SDK client itself.

## Evidence and ownership

Build each resource scenario from these sources, in order:

1. A sanitized recorded cassette, when available, establishes observed OCI
   methods, paths, status codes, headers, response shape, and lifecycle order.
2. The vendored OCI Go SDK establishes current request and response types,
   mandatory fields, enums, and HTTP serialization.
3. Repo-authored formal metadata establishes OSOK lifecycle, mutation,
   identity, follow-up, and deletion intent. Provider-fact imports are pinned
   by `formal/sources.lock` to `terraform-provider-oci`.
4. The pinned Terraform provider implementation is supporting evidence for
   operation selection, request mapping, mutable versus force-new fields,
   waiters, and response flattening.
5. Synthetic cassettes supply contract examples only where recorded evidence
   is unavailable; they are not proof of live OCI behavior.

The OCI SDK remains authoritative for SDK shapes. Terraform provider behavior
must agree with formal imports and repo-authored OSOK semantics, but it does not
replace the typed SDK response contract.

Shared transport, CRUD routing, and lifecycle orchestration live under
`internal/integration/ocimock`. Resource scenarios stay beside their service
manager as `*_mock_integration_test.go`, where they can use package-private
runtime hooks without exporting production APIs only for tests.

## Synchronous CRUD groups

The initial generated-runtime inventory contains 412 CRUD-capable resources.
Eighty-seven use a work-request or other explicitly asynchronous resource-local
path and are deferred to the asynchronous phase. The remaining 325 synchronous
candidates are divided into:

| Group | Count | Contract |
| --- | ---: | --- |
| S1: immediate top-level | 8 | One collection and item route with an explicit reviewed synchronous semantic contract; a steady response can complete create/update immediately. |
| Needs contract classification | 73 | No work request or composite path is visible, but the resource has no explicit runtime semantics. Do not assume it is immediate; reconcile its recorded/SDK/provider behavior first. |
| S2: lifecycle-polled | 164 | No work request, but create/update/delete convergence depends on lifecycle-state reads. |
| S3: composite or nested path | 80 | Parent or composite path identity must be preserved across CRUD and deletion confirmation. |

Ownership and evidence are cross-cutting attributes: 123 of the synchronous
resources use only their generated baseline, 202 have resource-local production
override, 115 have recorded evidence, 210 are synthetic-only, and 90 currently
have formal catalog rows.

Start with S1, and classify evidence-backed resources from the unclassified
group before moving them into S1 or S2. Within each group, prefer recorded and formal-covered resources
first, then recorded without formal coverage, then synthetic-only resources.
Audit any resource-local override while migrating that resource.

## Scenario contract

Every CRUD scenario must prove both mapping directions:

```text
typed CR spec
  -> production service manager
  -> real OCI SDK HTTP request
  -> mock request assertions and typed dynamic state
  -> OCI-compatible HTTP response
  -> real OCI SDK typed response
  -> production status projection
  -> typed CR status assertions
```

Where formal metadata requires them, the responder also verifies read after
create, read after update, and post-delete read confirmation. New fields are
added explicitly to the typed CR fixture, expected SDK request, typed mock
response, and CR status assertion only when OSOK intentionally adopts them.

Run the current mock integration surface with:

```bash
make mockintegrationtest
```

The target first audits the source-derived inventory and fails if an explicitly
classified S1 resource lacks a package-local dynamic scenario. Resources with
missing semantic metadata stay in `needs-contract-classification` and cannot
silently satisfy the S1 coverage check.

All eight explicitly classified S1 resources have dynamic CRUD scenarios. The
references deliberately cover different evidence boundaries:

- `Budget` starts from recorded OCI evidence plus a seeded formal contract.
- `HttpMonitor` starts from recorded evidence plus a seeded formal contract;
  its resource-local runtime retains pagination and ambiguous-delete handling.
- `WlpAgent` starts from a synthetic SDK contract plus a seeded formal contract;
  its resource-local runtime retains only the state-free Active projection.
- `AutoScalingConfiguration` starts from synthetic evidence and demonstrates
  how migration must first correct missing formal semantics instead of treating
  absent metadata as proof of immediate behavior.
- Resource Manager `Template` starts from a real OCI recording and reviewed
  SDK-backed local semantics because the pinned Terraform provider exposes no
  corresponding resource implementation.

The S2 references include `SavedQuery`, Data Safe `SensitiveType`, the File
Storage `FileSystem`, `FilesystemSnapshotPolicy`, and `Snapshot` resources, and
Resource Manager `Stack`. `SavedQuery`'s refreshed live cassette verifies
the formal-backed full update request, the pre-delete state read, accepted
delete, and final 404 confirmation; its dynamic scenario additionally exercises
the provider-documented `CREATING` and `DELETING` transitions without cloud
latency. The newer S2 scenarios apply the same contract while distinguishing
formal provider-backed resources from SDK-and-recording-backed resources whose
Terraform provider has no matching resource implementation.
