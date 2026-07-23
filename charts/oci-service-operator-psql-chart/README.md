# OCI Service Operator PostgreSQL chart

This directory is the reviewed source skeleton for
`oci-service-operator-psql-chart`. Do not package it directly:
`make package-helm` injects the release version and controller image, copies the
generated PostgreSQL RBAC and CRD, validates manifest parity, and creates the
release archive.

The chart intentionally does not create a Namespace. All namespaced resources
use the Helm release namespace.
