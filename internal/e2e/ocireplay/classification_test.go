/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"fmt"
	"strings"
	"testing"
)

func TestAuditCoverageAppliesClassificationInventory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, controller := range []string{
		"controllers/aivision/project_controller.go",
		"controllers/budget/budget_controller.go",
		"controllers/database/autonomousdatabase_controller.go",
		"controllers/functions/function_controller.go",
		"controllers/ons/topic_controller.go",
	} {
		writeCoverageFile(t, root, controller, "package controller\n")
	}
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/project_synthetic_integration_test.go",
		"package project\n// project.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/testdata/recordings/project.yaml",
		classificationCassette("aivision", "Project", ProvenanceSynthetic),
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/budget/budget/budget_recorded_integration_test.go",
		"package budget\n// budget.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/budget/budget/testdata/recordings/budget.yaml",
		classificationCassette("budget", "Budget", ProvenanceRecorded),
	)
	writeCoverageFile(t, root, coverageClassificationPath, `schemaVersion: 1
resources:
  - service: aivision
    resource: project
    classification: synthetic
    reason: Live creation consumes paid model-serving capacity.
  - service: database
    resource: autonomousdatabase
    classification: synthetic
    reason: Live creation provisions paid database compute and storage.
    nextAction: Add a synthetic lifecycle cassette.
  - service: functions
    resource: function
    classification: deferred
    reason: The live test requires an invocation image.
    blocker: No reusable image fixture is packaged.
    nextAction: Publish an image fixture and record the lifecycle.
`)

	report, err := AuditCoverage(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalControllers != 5 || report.CoveredResources != 2 ||
		report.ClassifiedResources != 4 || report.RecordedClassified != 1 ||
		report.SyntheticClassified != 2 || report.DeferredClassified != 1 ||
		len(report.Unclassified) != 1 {
		t.Fatalf("classification report = %+v", report)
	}
	if len(report.Deferred) != 1 || report.Deferred[0].Service != "functions" ||
		report.Deferred[0].Blocker == "" || report.Deferred[0].NextAction == "" {
		t.Fatalf("deferred = %+v", report.Deferred)
	}
	if len(report.Missing) != 3 {
		t.Fatalf("missing = %+v, want three uncovered controllers", report.Missing)
	}
}

func TestAuditCoverageRequiresClassificationInventory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/ons/topic_controller.go", "package ons\n")
	writeCoverageFile(t, root, "pkg/servicemanager/.keep", "")

	_, err := AuditCoverage(root)
	if err == nil || !strings.Contains(err.Error(), "read required replay classifications") {
		t.Fatalf("AuditCoverage() error = %v, want required inventory error", err)
	}
}

func TestAuditCoverageRequiresSyntheticJustificationWhenInventoryExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/aivision/project_controller.go", "package aivision\n")
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/project_synthetic_integration_test.go",
		"package project\n// project.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/testdata/recordings/project.yaml",
		classificationCassette("aivision", "Project", ProvenanceSynthetic),
	)
	writeCoverageFile(t, root, coverageClassificationPath, "schemaVersion: 1\nresources: []\n")

	_, err := AuditCoverage(root)
	if err == nil || !strings.Contains(err.Error(), "requires a classification justification") {
		t.Fatalf("AuditCoverage() error = %v, want missing synthetic justification", err)
	}
}

func TestAuditCoverageRequiresSyntheticScenarioReasonForRecordedResource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/objectstorage/bucket_controller.go", "package objectstorage\n")
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/objectstorage/bucket/bucket_recorded_integration_test.go",
		"package bucket\n// recorded.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/objectstorage/bucket/bucket_synthetic_integration_test.go",
		"package bucket\n// synthetic.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/objectstorage/bucket/testdata/recordings/recorded.yaml",
		classificationCassette("objectstorage", "Bucket", ProvenanceRecorded),
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/objectstorage/bucket/testdata/recordings/synthetic.yaml",
		classificationCassette("objectstorage", "Bucket", ProvenanceSynthetic),
	)
	writeCoverageFile(t, root, coverageClassificationPath, `schemaVersion: 1
resources:
  - service: objectstorage
    resource: bucket
    classification: recorded
`)

	_, err := AuditCoverage(root)
	if err == nil || !strings.Contains(err.Error(), "requires syntheticReason") {
		t.Fatalf("AuditCoverage() error = %v, want synthetic scenario justification", err)
	}
}

func TestAuditCoverageRejectsClassificationThatConflictsWithCassetteProvenance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCoverageFile(t, root, "controllers/aivision/project_controller.go", "package aivision\n")
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/project_synthetic_integration_test.go",
		"package project\n// project.yaml\n",
	)
	writeCoverageFile(
		t,
		root,
		"pkg/servicemanager/aivision/project/testdata/recordings/project.yaml",
		classificationCassette("aivision", "Project", ProvenanceSynthetic),
	)
	writeCoverageFile(t, root, coverageClassificationPath, `schemaVersion: 1
resources:
  - service: aivision
    resource: project
    classification: recorded
`)

	_, err := AuditCoverage(root)
	if err == nil || !strings.Contains(err.Error(), "cassette coverage derives \"synthetic\"") {
		t.Fatalf("AuditCoverage() error = %v, want cassette strategy conflict", err)
	}
}

func TestAuditCoverageRejectsInvalidClassificationInventory(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		inventory string
		want      string
	}{
		{
			name: "unknown controller",
			inventory: `schemaVersion: 1
resources:
  - service: unknown
    resource: resource
    classification: synthetic
    reason: unavailable
    nextAction: add replay
`,
			want: "unknown controllers",
		},
		{
			name: "duplicate",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: recorded
    nextAction: record it
  - service: ons
    resource: topic
    classification: recorded
    nextAction: record it
`,
			want: "declared more than once",
		},
		{
			name: "deferred missing blocker",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: deferred
    reason: blocked
    nextAction: resolve it
`,
			want: "requires reason, blocker, and nextAction",
		},
		{
			name: "planned recorded missing action",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: recorded
`,
			want: "requires nextAction",
		},
		{
			name: "synthetic missing reason",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: synthetic
    nextAction: add a synthetic replay
`,
			want: "requires reason",
		},
		{
			name: "unclassified declaration",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: unclassified
`,
			want: "unclassified is derived",
		},
		{
			name: "unknown field",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: recorded
    nextAction: record it
    unexpected: true
`,
			want: "field unexpected not found",
		},
		{
			name: "unsorted declarations",
			inventory: `schemaVersion: 1
resources:
  - service: ons
    resource: topic
    classification: recorded
    nextAction: record it
  - service: adm
    resource: knowledgebase
    classification: recorded
    nextAction: record it
`,
			want: "must be sorted",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCoverageFile(t, root, "controllers/ons/topic_controller.go", "package ons\n")
			writeCoverageFile(t, root, "pkg/servicemanager/.keep", "")
			writeCoverageFile(t, root, coverageClassificationPath, testCase.inventory)
			_, err := AuditCoverage(root)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("AuditCoverage() error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func classificationCassette(service string, resource string, provenance Provenance) string {
	return fmt.Sprintf(`version: 1
metadata:
  service: %s
  resource: %s
  operations: [read]
  sdkVersion: v65.110.0
  provenance: %s
interactions:
  - request: {method: GET, host: example.test, path: /resource}
    response: {statusCode: 200}
`, service, resource, provenance)
}
