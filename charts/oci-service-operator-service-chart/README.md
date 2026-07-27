# OCI Service Operator __OSOK_GROUP__ chart

This directory is the reviewed source skeleton for
`__OSOK_CHART_NAME__`. Do not package it directly:
`make package-helm` injects the release version and controller image, copies the
generated __OSOK_GROUP__ RBAC and CRDs, validates manifest parity, and creates the
release archive.

The chart intentionally does not create a Namespace. All namespaced resources
use the Helm release namespace.
