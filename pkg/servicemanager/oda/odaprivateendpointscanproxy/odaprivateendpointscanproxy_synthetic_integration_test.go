/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package odaprivateendpointscanproxy

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOdaPrivateEndpointScanProxyReadsTracked(t *testing.T) {
	resource := &odav1beta1.OdaPrivateEndpointScanProxy{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		OdaPrivateEndpointIDAnnotation: "ocid1.odaprivateendpoint.oc1..synthetic",
	}}}
	resourceID := "ocid1.odaprivateendpointscanproxy.oc1..synthetic"
	inputValues := map[string]any{"odaPrivateEndpointId": "ocid1.odaprivateendpoint.oc1..synthetic"}
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	resource.Spec.ScanListenerType = "FQDN"
	resource.Spec.Protocol = "TCP"
	resource.Spec.ScanListenerInfos = []odav1beta1.OdaPrivateEndpointScanProxyScanListenerInfo{{ScanListenerFqdn: "scan.example.invalid", ScanListenerPort: 1521}}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "odaPrivateEndpointId": "ocid1.odaprivateendpoint.oc1..synthetic", "protocol": "TCP", "resourceId": resourceID, "scanListenerInfos": []map[string]any{{"scanListenerFqdn": "scan.example.invalid", "scanListenerPort": 1521}}, "scanListenerType": "FQDN", "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "OdaPrivateEndpointScanProxy", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "odaprivateendpointscanproxy_synthetic_read.yaml"), Host: "https://digitalassistant-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newOdaPrivateEndpointScanProxyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked OdaPrivateEndpointScanProxy response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic OdaPrivateEndpointScanProxy cassette: %w", err))
	}
}
