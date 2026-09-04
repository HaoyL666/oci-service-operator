/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package subscription

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	osubsubscriptionsdk "github.com/oracle/oci-go-sdk/v65/osubsubscription"
	osubsubscriptionv1beta1 "github.com/oracle/oci-service-operator/api/osubsubscription/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOsubSubscriptionReadsExisting(t *testing.T) {
	resource := &osubsubscriptionv1beta1.Subscription{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"subscription","namespace":"default"},"spec":{"compartmentId":"ocid1.compartment.oc1..replay","subscriptionId":"subscription-1"}}`), resource); err != nil {
		t.Fatal(err)
	}
	observed := `[{"id":"subscription-1","planNumber":"plan-1","status":"ACTIVE"}]`
	metadata := ocireplay.Metadata{Service: "osubsubscription", Resource: "Subscription", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "subscription_synthetic_read.yaml"), Host: "https://subscription.us-ashburn-1.oci.oraclecloud.com", BasePath: "oalapp/service/onesubs/proxy/20210501", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := osubsubscriptionsdk.SubscriptionClient{BaseClient: session.BaseClient()}
	client := newSubscriptionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Status != "ACTIVE" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
}
