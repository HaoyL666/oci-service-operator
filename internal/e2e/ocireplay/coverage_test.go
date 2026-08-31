/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditCoverageReportsCoveredMissingLegacyAndOrphanResources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/aivision/project_controller.go", "package aivision\n")
	writeCoverageFile(t, root, "controllers/budget/budget_controller.go", "package budget\n")
	writeCoverageFile(t, root, "controllers/ons/topic_controller.go", "package ons\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/project_synthetic_integration_test.go", "package project\n// project.yaml\n")
	writeCoverageFile(t, root, "pkg/servicemanager/budget/budget/budget_recorded_integration_test.go", "package budget\n// budget.yaml\n")
	writeCoverageFile(t, root, "pkg/servicemanager/orphan/resource/resource_synthetic_integration_test.go", "package resource\n// orphan.yaml\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/testdata/recordings/project.yaml", `version: 1
metadata:
  service: aivision
  resource: Project
  operations: [create, read, delete]
  sdkVersion: v65.110.0
  provenance: synthetic
interactions:
  - request: {method: GET, host: example.test, path: /projects}
    response: {statusCode: 200}
`)
	writeCoverageFile(t, root, "pkg/servicemanager/budget/budget/testdata/recordings/budget.yaml", `version: 1
metadata:
  service: budget
  resource: Budget
  operations: [create, read, update, delete]
  sdkVersion: v65.110.0
  provenance: recorded
interactions:
  - request: {method: GET, host: example.test, path: /budgets}
    response: {statusCode: 200}
`)
	writeCoverageFile(t, root, "pkg/servicemanager/legacy/resource/testdata/recordings/legacy.yaml", `version: 1
interactions:
  - request: {method: GET, host: example.test, path: /legacy}
    response: {statusCode: 200}
`)
	writeCoverageFile(t, root, "pkg/servicemanager/orphan/resource/testdata/recordings/orphan.yaml", `version: 1
metadata:
  service: orphan
  resource: Resource
  operations: [read]
  sdkVersion: v65.110.0
  provenance: synthetic
interactions:
  - request: {method: GET, host: example.test, path: /orphan}
    response: {statusCode: 200}
`)
	writeCoverageFile(t, root, "pkg/servicemanager/unused/resource/testdata/recordings/unused.yaml", `version: 1
metadata:
  service: unused
  resource: Resource
  operations: [read]
  sdkVersion: v65.110.0
  provenance: synthetic
interactions:
  - request: {method: GET, host: example.test, path: /unused}
    response: {statusCode: 200}
`)

	report, err := AuditCoverage(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalControllers != 3 || report.CoveredResources != 2 || report.RecordedResources != 1 || report.SyntheticResources != 1 {
		t.Fatalf("coverage totals = %+v", report)
	}
	if len(report.Missing) != 1 || report.Missing[0].Service != "ons" || report.Missing[0].Resource != "topic" {
		t.Fatalf("missing = %+v", report.Missing)
	}
	if len(report.LegacyCassettes) != 1 || report.LegacyCassettes[0] != "pkg/servicemanager/legacy/resource/testdata/recordings/legacy.yaml" {
		t.Fatalf("legacy = %+v", report.LegacyCassettes)
	}
	if len(report.UnreferencedCassettes) != 1 || report.UnreferencedCassettes[0] != "pkg/servicemanager/unused/resource/testdata/recordings/unused.yaml" {
		t.Fatalf("unreferenced = %+v", report.UnreferencedCassettes)
	}
	if len(report.OrphanCassettes) != 1 || report.OrphanCassettes[0].Metadata.Service != "orphan" {
		t.Fatalf("orphans = %+v", report.OrphanCassettes)
	}
	var budgetCoverage *CoverageResource
	for index := range report.Resources {
		if report.Resources[index].Service == "budget" {
			budgetCoverage = &report.Resources[index]
			break
		}
	}
	if budgetCoverage == nil || len(budgetCoverage.Cassettes) != 1 || budgetCoverage.Cassettes[0].Metadata.Provenance != ProvenanceRecorded {
		t.Fatalf("budget coverage = %+v", budgetCoverage)
	}
}

func TestAuditCoverageIgnoresCassetteReferencedOnlyByOrdinaryTest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/aivision/project_controller.go", "package aivision\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/project_test.go", "package project\n// project.yaml\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/testdata/recordings/project.yaml", `version: 1
metadata:
  service: aivision
  resource: Project
  operations: [read]
  sdkVersion: v65.110.0
  provenance: synthetic
interactions:
  - request: {method: GET, host: example.test, path: /projects/example}
    response: {statusCode: 200}
`)

	report, err := AuditCoverage(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.CoveredResources != 0 || len(report.UnreferencedCassettes) != 1 {
		t.Fatalf("coverage report = %+v", report)
	}
}

func TestAuditCoverageRequiresTestNameToMatchCassetteProvenance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/aivision/project_controller.go", "package aivision\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/project_recorded_integration_test.go", "package project\n// project.yaml\n")
	writeCoverageFile(t, root, "pkg/servicemanager/aivision/project/testdata/recordings/project.yaml", `version: 1
metadata:
  service: aivision
  resource: Project
  operations: [read]
  sdkVersion: v65.110.0
  provenance: synthetic
interactions:
  - request: {method: GET, host: example.test, path: /projects/example}
    response: {statusCode: 200}
`)

	report, err := AuditCoverage(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.CoveredResources != 0 || report.SyntheticResources != 0 ||
		len(report.UnreferencedCassettes) != 1 {
		t.Fatalf("coverage report = %+v", report)
	}
}

func TestAuditCoverageRejectsMalformedCassette(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/budget/budget_controller.go", "package budget\n")
	writeCoverageFile(t, root, "pkg/servicemanager/budget/budget/testdata/recordings/budget.yaml", "version: 1\nmetadata: {}\n")
	if _, err := AuditCoverage(root); err == nil {
		t.Fatal("AuditCoverage() error = nil, want malformed cassette error")
	}
}

func writeCoverageFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
