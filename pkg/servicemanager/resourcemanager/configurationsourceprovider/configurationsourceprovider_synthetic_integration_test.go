/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package configurationsourceprovider

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
	resourcemanagerv1beta1 "github.com/oracle/oci-service-operator/api/resourcemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticConfigurationSourceProviderReconcilesTracked(t *testing.T) {
	resource := &resourcemanagerv1beta1.ConfigurationSourceProvider{}
	resourceID := "ocid1.configurationsourceprovider.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"configSourceProviderType": "GITHUB_ACCESS_TOKEN", "key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "resourcemanager", Resource: "ConfigurationSourceProvider", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "configurationsourceprovider_synthetic_reconcile.yaml"), Host: "https://resourcemanager.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180917", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := resourcemanagersdk.ResourceManagerClient{BaseClient: session.BaseClient()}
	manager := &ConfigurationSourceProviderServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newConfigurationSourceProviderDefaultRuntimeHooks(sdkClient)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapConfigurationSourceProviderGeneratedClient(hooks, defaultConfigurationSourceProviderServiceClient{ServiceClient: generatedruntime.NewServiceClient[*resourcemanagerv1beta1.ConfigurationSourceProvider](buildConfigurationSourceProviderGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked ConfigurationSourceProvider response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic ConfigurationSourceProvider cassette: %w", err))
	}
}
