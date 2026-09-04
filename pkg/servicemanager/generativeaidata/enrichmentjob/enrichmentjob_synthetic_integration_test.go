/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package enrichmentjob

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	generativeaidatasdk "github.com/oracle/oci-go-sdk/v65/generativeaidata"
	generativeaidatav1beta1 "github.com/oracle/oci-service-operator/api/generativeaidata/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticEnrichmentJobCreateReadDelete(t *testing.T) {
	resource := makeEnrichmentJobResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.enrichmentjob.oc1..synthetic", "SUCCEEDED", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "generativeaidata", Resource: "EnrichmentJob", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "enrichmentjob_synthetic_crud.yaml"), Host: "https://generative-ai-data.us-ashburn-1.oci.oraclecloud.com", BasePath: "20260325", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	sdkClient := EnrichmentJobSDKClients{
		generateEnrichmentJobClient: generativeaidatasdk.GenerateEnrichmentJobClient{BaseClient: baseClient},
		getEnrichmentJobClient:      generativeaidatasdk.GetEnrichmentJobClient{BaseClient: baseClient},
		listEnrichmentJobsClient:    generativeaidatasdk.ListEnrichmentJobsClient{BaseClient: baseClient},
		cancelEnrichmentJobClient:   generativeaidatasdk.CancelEnrichmentJobClient{BaseClient: baseClient},
	}
	client := newEnrichmentJobServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaidatav1beta1.EnrichmentJob]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *generativeaidatav1beta1.EnrichmentJob) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *generativeaidatav1beta1.EnrichmentJob) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created EnrichmentJob status = %+v", current.Status)
			}
			return nil
		},
	})
}
