---
schemaVersion: 1
surface: repo-authored-semantics
service: objectstorage
slug: bucket
gaps:
  - category: legacy-adapter
    status: resolved
    stopCondition: "generatedruntime can populate Bucket path fields from the published Bucket surface (`bucketName` and `namespaceName`) and model synchronous create or update responses without lifecycle states, so pkg/servicemanager/objectstorage/bucket no longer needs the handwritten adapter."
---

# Logic Gaps

## Current runtime path

- `Bucket` now routes through the generated-runtime client in
  `pkg/servicemanager/objectstorage/bucket/bucket_serviceclient.go`.
- Generatedruntime resolves the `bucketName` and `namespaceName` request paths
  from the published `Bucket` surface, preferring observed
  `status.name`/`status.namespace` and falling back to
  `spec.name`/`spec.namespace` when the bucket has not been observed yet.
- Successful create and update responses use read-after-write follow-up and
  treat missing lifecycle fields as synchronous success, projecting the
  returned bucket directly into `status` and marking the resource `Active`.
- Delete confirmation remains explicit. The finalizer stays until
  `DeleteBucket` succeeds and a follow-up `GetBucket` confirms the bucket is
  gone.

## Repo-authored semantics

- `spec.namespace`, not the Kubernetes object namespace, is the OCI namespace
  input for create, and it remains the fallback identity source until the
  controller has observed the OCI bucket.
- Create or bind is explicit: generatedruntime first reads the observed bucket
  using the published namespace and name, binds when the bucket already exists,
  and creates the bucket only when OCI returns `NotFound`.
- Update and delete target the currently observed OCI bucket identity when one
  is recorded, so rename flows mutate the existing bucket instead of issuing
  path requests against the desired post-update name.
- `storageTier` is treated as create-only drift. Changing it after the bucket
  exists returns an explicit error instead of silently widening update
  behavior.
- No Kubernetes secret reads or writes are part of the bucket path.
