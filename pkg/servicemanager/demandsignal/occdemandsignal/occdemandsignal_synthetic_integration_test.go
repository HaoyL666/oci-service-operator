/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package occdemandsignal

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	demandsignalsdk "github.com/oracle/oci-go-sdk/v65/demandsignal"
	demandsignalv1beta1 "github.com/oracle/oci-service-operator/api/demandsignal/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOccDemandSignalReconcilesTracked(t *testing.T) {
	resource := &demandsignalv1beta1.OccDemandSignal{}
	resourceID := "ocid1.occdemandsignal.oc1..synthetic"
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, nil); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(resource.Spec, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "timeScheduleStart": "2026-01-02T03:04:05Z", "timeReleased": "2026-01-02T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "demandsignal", Resource: "OccDemandSignal", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "occdemandsignal_synthetic_reconcile.yaml"), Host: "https://control-center-ds.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240430", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := demandsignalsdk.OccDemandSignalClient{BaseClient: session.BaseClient()}
	manager := &OccDemandSignalServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newOccDemandSignalRuntimeHooks(manager, sdkClient)
	client := wrapOccDemandSignalGeneratedClient(hooks, defaultOccDemandSignalServiceClient{ServiceClient: generatedruntime.NewServiceClient[*demandsignalv1beta1.OccDemandSignal](buildOccDemandSignalGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked OccDemandSignal response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic OccDemandSignal cassette: %w", err))
	}
}
