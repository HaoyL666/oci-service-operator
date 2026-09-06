/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package scheduledquery

import (
	"path/filepath"
	"testing"

	apmtracessdk "github.com/oracle/oci-go-sdk/v65/apmtraces"
	apmtracesv1beta1 "github.com/oracle/oci-service-operator/api/apmtraces/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationScheduledQueryLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "scheduledquery_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ScheduledQuery OCI mock: %v", err)
		}
	})
	domainID := "ocid1.apmdomain.oc1..replay"
	resource := &apmtracesv1beta1.ScheduledQuery{
		Spec: apmtracesv1beta1.ScheduledQuerySpec{
			ApmDomainId:                           domainID,
			ScheduledQueryName:                    "osok-replay-scheduled-query",
			ScheduledQueryProcessingType:          "QUERY",
			ScheduledQueryProcessingSubType:       "NONE",
			ScheduledQueryText:                    "SHOW SPANS * FIRST 100 ROWS BETWEEN now() - 2 HOURS AND now()",
			ScheduledQuerySchedule:                "SCHEDULE STARTING AFTER 2099-01-01T00:00:00Z EVERY 720 MINUTES",
			ScheduledQueryDescription:             "OSOK synthetic scheduled query",
			ScheduledQueryMaximumRuntimeInSeconds: 60,
			ScheduledQueryRetentionCriteria:       "KEEP_DATA_UNTIL_RETENTION_PERIOD",
			ScheduledQueryRetentionPeriodInMs:     86400000,
			FreeformTags:                          map[string]string{"osok-replay": "create"},
		},
	}
	ocimock.InitializeResource(resource, "mock-scheduledquery")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.ApmDomainId = domainID
	sdkClient := apmtracessdk.ScheduledQueryClient{BaseClient: session.BaseClient()}
	client := newScheduledQueryServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
