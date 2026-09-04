/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package serviceattachment

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	fusionappssdk "github.com/oracle/oci-go-sdk/v65/fusionapps"
	fusionappsv1beta1 "github.com/oracle/oci-service-operator/api/fusionapps/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticServiceAttachmentReadsTracked(t *testing.T) {
	resource := &fusionappsv1beta1.ServiceAttachment{}
	resourceID := "ocid1.serviceattachment.oc1..synthetic"
	pathValues := map[string]any{"fusionEnvironmentId": "ocid1.fusionenvironment.oc1..synthetic"}
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, pathValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "fusionEnvironmentId": "ocid1.fusionenvironment.oc1..synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "fusionapps", Resource: "ServiceAttachment", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "serviceattachment_synthetic_read.yaml"), Host: "https://fusionapps.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211201", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := fusionappssdk.FusionApplicationsClient{BaseClient: session.BaseClient()}
	manager := &ServiceAttachmentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newServiceAttachmentDefaultRuntimeHooks(sdkClient)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	client := wrapServiceAttachmentGeneratedClient(hooks, defaultServiceAttachmentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*fusionappsv1beta1.ServiceAttachment](buildServiceAttachmentGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked ServiceAttachment response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic ServiceAttachment cassette: %w", err))
	}
}
