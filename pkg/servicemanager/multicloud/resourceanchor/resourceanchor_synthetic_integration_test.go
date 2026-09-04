/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package resourceanchor

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

func TestSyntheticResourceAnchorReadsTracked(t *testing.T) {
	resource := &multicloudv1beta1.ResourceAnchor{ObjectMeta: metav1.ObjectMeta{
		Name: "synthetic-resource-anchor",
		Annotations: map[string]string{
			resourceAnchorIDAnnotation:                      "ocid1.resourceanchor.oc1..synthetic",
			resourceAnchorSubscriptionIDAnnotation:          "ocid1.multicloudsubscription.oc1..synthetic",
			resourceAnchorSubscriptionServiceNameAnnotation: "ORACLEDBATAZURE",
		},
	}}
	resourceID := "ocid1.resourceanchor.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"displayName": "synthetic-resource-anchor", "key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "subscriptionId": "ocid1.multicloudsubscription.oc1..synthetic", "subscriptionType": "ORACLEDBATAZURE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "multicloud", Resource: "ResourceAnchor", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "resourceanchor_synthetic_read.yaml"), Host: "https://multicloud.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180828", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := multicloudsdk.OmhubResourceAnchorClient{BaseClient: session.BaseClient()}
	client := resourceAnchorReadOnlyClient{
		get:  sdkClient.GetResourceAnchor,
		list: sdkClient.ListResourceAnchors,
		log:  loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked ResourceAnchor response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic ResourceAnchor cassette: %w", err))
	}
}
