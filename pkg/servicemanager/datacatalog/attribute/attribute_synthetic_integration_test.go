/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package attribute

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	datacatalogsdk "github.com/oracle/oci-go-sdk/v65/datacatalog"
	datacatalogv1beta1 "github.com/oracle/oci-service-operator/api/datacatalog/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAttributeReconcilesTracked(t *testing.T) {
	resource := &datacatalogv1beta1.Attribute{}
	resourceID := "ocid1.attribute.oc1..synthetic"
	pathValues := map[string]any{"catalogId": "ocid1.catalog.oc1..synthetic", "dataAssetKey": "dataassetkey-synthetic", "entityKey": "entitykey-synthetic", "timeExternal": "2026-01-02T03:04:05Z"}
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, pathValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "catalogId": "ocid1.catalog.oc1..synthetic", "dataAssetKey": "dataassetkey-synthetic", "entityKey": "entitykey-synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "datacatalog", Resource: "Attribute", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "attribute_synthetic_reconcile.yaml"), Host: "https://datacatalog.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190325", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datacatalogsdk.DataCatalogClient{BaseClient: session.BaseClient()}
	manager := &AttributeServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAttributeDefaultRuntimeHooks(sdkClient)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapAttributeGeneratedClient(hooks, defaultAttributeServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.Attribute](buildAttributeGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked Attribute response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic Attribute cassette: %w", err))
	}
}
