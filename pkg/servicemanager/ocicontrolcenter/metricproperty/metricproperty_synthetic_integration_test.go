/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package metricproperty

import (
	"context"
	"path/filepath"
	"testing"

	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMetricPropertyReadsExisting(t *testing.T) {
	resource := newMetricPropertyTestResource()
	itemBody, err := ocireplay.SyntheticJSONBody(metricPropertySummary(testMetricPropertyMetricName, map[string]ocicontrolcentersdk.DimensionValue{}))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "ocicontrolcenter", Resource: "MetricProperty", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	collectionPath := "/20230515/metricProperties/" + testMetricPropertyNamespaceName
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "metricproperty_synthetic_read.yaml"), Host: "https://control-center.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230515", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CollectionPath: collectionPath, InitiallyPresent: true, PresentCollectionBody: "{\"items\":[" + itemBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := ocicontrolcentersdk.OccMetricsClient{BaseClient: session.BaseClient()}
	client := newMetricPropertyRuntimeClient(&MetricPropertyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}, sdkClient, nil)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
	deleted, err := client.Delete(context.Background(), resource)
	if err != nil || !deleted {
		t.Fatalf("read-only delete = (%t, %v)", deleted, err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
