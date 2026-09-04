/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package autoscalingpolicy

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	autoscalingsdk "github.com/oracle/oci-go-sdk/v65/autoscaling"
	autoscalingv1beta1 "github.com/oracle/oci-service-operator/api/autoscaling/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAutoScalingPolicyReconcilesTracked(t *testing.T) {
	resource := &autoscalingv1beta1.AutoScalingPolicy{}
	resourceID := "ocid1.autoscalingpolicy.oc1..synthetic"
	pathValues := map[string]any{"autoScalingConfigurationId": "ocid1.autoscalingconfiguration.oc1..synthetic"}
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, pathValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "autoScalingConfigurationId": "ocid1.autoscalingconfiguration.oc1..synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "autoscaling", Resource: "AutoScalingPolicy", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "autoscalingpolicy_synthetic_reconcile.yaml"), Host: "https://autoscaling.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181001", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := autoscalingsdk.AutoScalingClient{BaseClient: session.BaseClient()}
	manager := &AutoScalingPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAutoScalingPolicyDefaultRuntimeHooks(sdkClient)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapAutoScalingPolicyGeneratedClient(hooks, defaultAutoScalingPolicyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*autoscalingv1beta1.AutoScalingPolicy](buildAutoScalingPolicyGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked AutoScalingPolicy response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic AutoScalingPolicy cassette: %w", err))
	}
}
