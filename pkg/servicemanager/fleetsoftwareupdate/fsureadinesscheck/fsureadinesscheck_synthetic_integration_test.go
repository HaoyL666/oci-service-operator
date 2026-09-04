/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package fsureadinesscheck

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	fleetsoftwareupdatesdk "github.com/oracle/oci-go-sdk/v65/fleetsoftwareupdate"
	fleetsoftwareupdatev1beta1 "github.com/oracle/oci-service-operator/api/fleetsoftwareupdate/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticFsuReadinessCheckReconcilesTracked(t *testing.T) {
	resource := &fleetsoftwareupdatev1beta1.FsuReadinessCheck{}
	resourceID := "ocid1.fsureadinesscheck.oc1..synthetic"
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, nil); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(resource.Spec, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "timeScheduleStart": "2026-01-02T03:04:05Z", "timeReleased": "2026-01-02T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "fleetsoftwareupdate", Resource: "FsuReadinessCheck", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "fsureadinesscheck_synthetic_reconcile.yaml"), Host: "https://fleet-software-update.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220528", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := fleetsoftwareupdatesdk.FleetSoftwareUpdateClient{BaseClient: session.BaseClient()}
	manager := &FsuReadinessCheckServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newFsuReadinessCheckRuntimeHooks(manager, sdkClient)
	client := wrapFsuReadinessCheckGeneratedClient(hooks, defaultFsuReadinessCheckServiceClient{ServiceClient: generatedruntime.NewServiceClient[*fleetsoftwareupdatev1beta1.FsuReadinessCheck](buildFsuReadinessCheckGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked FsuReadinessCheck response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic FsuReadinessCheck cassette: %w", err))
	}
}
