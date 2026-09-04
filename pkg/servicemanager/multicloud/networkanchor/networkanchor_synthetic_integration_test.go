/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkanchor

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticNetworkAnchorReadsTracked(t *testing.T) {
	resource := &multicloudv1beta1.NetworkAnchor{ObjectMeta: metav1.ObjectMeta{
		Name: "synthetic-network-anchor",
		Annotations: map[string]string{
			networkAnchorIDAnnotation:                  "ocid1.networkanchor.oc1..synthetic",
			networkAnchorSubscriptionIDAnnotation:      "ocid1.multicloudsubscription.oc1..synthetic",
			networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
		},
	}}
	resourceID := "ocid1.networkanchor.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "", map[string]any{"displayName": "synthetic-network-anchor", "key": resourceID, "networkAnchorLifecycleState": "ACTIVE", "resourceId": resourceID, "status": "ACTIVE", "subscriptionId": "ocid1.multicloudsubscription.oc1..synthetic", "subscriptionType": "ORACLEDBATAZURE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "multicloud", Resource: "NetworkAnchor", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "networkanchor_synthetic_read.yaml"), Host: "https://multicloud.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180828", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := multicloudsdk.OmhubNetworkAnchorClient{BaseClient: session.BaseClient()}
	client := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient, nil)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked NetworkAnchor response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic NetworkAnchor cassette: %w", err))
	}
}
