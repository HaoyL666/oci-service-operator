/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package zprpolicy

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	zprsdk "github.com/oracle/oci-go-sdk/v65/zpr"
	zprv1beta1 "github.com/oracle/oci-service-operator/api/zpr/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticZprPolicyReadsTracked(t *testing.T) {
	resource := &zprv1beta1.ZprPolicy{}
	resourceID := "ocid1.zprpolicy.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "zpr", Resource: "ZprPolicy", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "zprpolicy_synthetic_read.yaml"), Host: "https://zpr.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240301", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := zprsdk.ZprClient{BaseClient: session.BaseClient()}
	manager := &ZprPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newZprPolicyDefaultRuntimeHooks(sdkClient)
	applyZprPolicyRuntimeHooks(&hooks, sdkClient, nil)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapZprPolicyGeneratedClient(hooks, defaultZprPolicyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*zprv1beta1.ZprPolicy](buildZprPolicyGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked ZprPolicy response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic ZprPolicy cassette: %w", err))
	}
}
