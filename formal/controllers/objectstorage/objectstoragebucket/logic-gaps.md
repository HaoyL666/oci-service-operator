---
schemaVersion: 1
surface: repo-authored-semantics
service: objectstorage
slug: objectstoragebucket
gaps: []
---

# Logic Gaps

This scaffold row is the provider-facing `ObjectStorageBucket` alias, not the
published OSOK controller target. The repo-owned controller path is
`objectstorage/Bucket`; keep this row scaffold-only until there is a concrete
need to model the provider alias as a first-class surface.
