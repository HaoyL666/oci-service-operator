/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package externallocationsummariesmetadata

import (
	"context"
	"path/filepath"
	"testing"

	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticExternalLocationSummariesMetadataReadsExisting(t *testing.T) {
	resource := newExternalLocationSummariesMetadataResource()
	resource.Annotations[externalLocationSummariesMetadataSubscriptionIDAnnotation] = testExternalLocationSummariesMetadataSubscriptionID
	resource.Annotations[externalLocationSummariesMetadataEntityTypeAnnotation] = "dbsystem"
	itemBody, err := ocireplay.SyntheticJSONBody(map[string]any{"ociRegion": "us-ashburn-1"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "multicloud", Resource: "ExternalLocationSummariesMetadata", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "externallocationsummariesmetadata_synthetic_read.yaml"), Host: "https://multicloud.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180828", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CollectionPath: "/20180828/externalLocationSummariesMetadata", InitiallyPresent: true, PresentCollectionBody: "{\"items\":[" + itemBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := multicloudsdk.MetadataClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newExternalLocationSummariesMetadataServiceClientWithListCall(log, sdkClient.ListExternalLocationSummariesMetadata, nil)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful {
		t.Fatalf("read response = %+v", response)
	}
	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("read-only delete was not acknowledged")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
