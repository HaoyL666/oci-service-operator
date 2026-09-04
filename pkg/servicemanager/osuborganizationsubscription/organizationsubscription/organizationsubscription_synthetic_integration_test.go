/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package organizationsubscription

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	osuborganizationsubscriptionsdk "github.com/oracle/oci-go-sdk/v65/osuborganizationsubscription"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOrganizationSubscriptionBindsExisting(t *testing.T) {
	resource := makeOrganizationSubscriptionResource()
	observed := `[{"id":"sub-active","serviceName":"COMPUTE","status":"ACTIVE"}]`
	metadata := ocireplay.Metadata{Service: "osuborganizationsubscription", Resource: "OrganizationSubscription", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "organizationsubscription_synthetic_read.yaml"), Host: "https://organizationsubscription.us-ashburn-1.oci.oraclecloud.com", BasePath: "oalapp/service/onesubs/proxy/20210501", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := osuborganizationsubscriptionsdk.OrganizationSubscriptionClient{BaseClient: session.BaseClient()}
	client := newOrganizationSubscriptionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || string(resource.Status.OsokStatus.Ocid) != "sub-active" {
		t.Fatalf("bind response=%+v status=%+v", response, resource.Status)
	}
}
