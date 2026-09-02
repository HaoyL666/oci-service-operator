/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package scheduledquery

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	apmtracesv1beta1 "github.com/oracle/oci-service-operator/api/apmtraces/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticScheduledQueryCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{
		Service:  "apmtraces",
		Resource: "ScheduledQuery",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	domainID := "ocid1.apmdomain.oc1..replay"
	sdkClient, closeSession := ocireplay.OpenAPMTracesSDK(
		t,
		ocireplay.ModeReplay,
		filepath.Join("testdata", "recordings", "scheduledquery_synthetic_crud.yaml"),
		metadata,
	)
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
	client := newScheduledQueryServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apmtracesv1beta1.ScheduledQuery]{
		Mode:          ocireplay.ModeReplay,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *apmtracesv1beta1.ScheduledQuery) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *apmtracesv1beta1.ScheduledQuery) error {
			if current.Status.ScheduledQueryName != "osok-replay-scheduled-query" || current.Status.ScheduledQueryText != "SHOW SPANS * FIRST 100 ROWS BETWEEN now() - 2 HOURS AND now()" {
				return fmt.Errorf("created ScheduledQuery status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *apmtracesv1beta1.ScheduledQuery) {
			current.Spec.ScheduledQueryDescription = "OSOK synthetic scheduled query updated"
			current.Spec.ScheduledQuerySchedule = "SCHEDULE STARTING AFTER 2099-01-01T00:00:00Z EVERY 360 MINUTES"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *apmtracesv1beta1.ScheduledQuery) error {
			if current.Status.ScheduledQueryDescription != "OSOK synthetic scheduled query updated" || current.Status.ScheduledQuerySchedule != "SCHEDULE STARTING AFTER 2099-01-01T00:00:00Z EVERY 360 MINUTES" {
				return fmt.Errorf("updated ScheduledQuery status = %+v", current.Status)
			}
			return nil
		},
	})
}
