/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package autoscalingconfiguration

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	autoscalingsdk "github.com/oracle/oci-go-sdk/v65/autoscaling"
	"github.com/oracle/oci-go-sdk/v65/common"
	autoscalingv1beta1 "github.com/oracle/oci-service-operator/api/autoscaling/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAutoScalingConfigurationReconcilesTracked(t *testing.T) {
	resource := newMockAutoScalingConfigurationResource()
	resourceID := "ocid1.autoscalingconfiguration.oc1..synthetic"
	resource.Status.Id = resourceID
	resource.Status.OsokStatus.Ocid = shared.OCID(resourceID)
	createdAt := common.SDKTime{Time: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}
	details := mockCreateAutoScalingConfigurationDetails(resource)
	observedBody, err := ocireplay.SyntheticJSONBody(autoscalingsdk.AutoScalingConfiguration{
		Id:                common.String(resourceID),
		CompartmentId:     details.CompartmentId,
		Resource:          details.Resource,
		Policies:          mockObservedAutoScalingPolicies(details.Policies, createdAt),
		TimeCreated:       &createdAt,
		DisplayName:       details.DisplayName,
		FreeformTags:      details.FreeformTags,
		CoolDownInSeconds: details.CoolDownInSeconds,
		IsEnabled:         details.IsEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "autoscaling", Resource: "AutoScalingConfiguration", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "autoscalingconfiguration_synthetic_reconcile.yaml"), Host: "https://autoscaling.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181001", Metadata: metadata, Bindings: map[string]string{"compartment": resource.Spec.CompartmentId, "instance-pool": mockInstancePoolID}, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := autoscalingsdk.AutoScalingClient{BaseClient: session.BaseClient()}
	manager := &AutoScalingConfigurationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAutoScalingConfigurationRuntimeHooks(manager, sdkClient)
	client := wrapAutoScalingConfigurationGeneratedClient(hooks, defaultAutoScalingConfigurationServiceClient{ServiceClient: generatedruntime.NewServiceClient[*autoscalingv1beta1.AutoScalingConfiguration](buildAutoScalingConfigurationGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked AutoScalingConfiguration response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic AutoScalingConfiguration cassette: %w", err))
	}
}
