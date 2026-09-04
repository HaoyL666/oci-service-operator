/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package datasource

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	managementagentsdk "github.com/oracle/oci-go-sdk/v65/managementagent"
	managementagentv1beta1 "github.com/oracle/oci-service-operator/api/managementagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDataSourceReadsTracked(t *testing.T) {
	resource := &managementagentv1beta1.DataSource{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		dataSourceManagementAgentIDAnnotation: "ocid1.managementagent.oc1..synthetic",
	}}}
	resourceID := "ocid1.datasource.oc1..synthetic"
	inputValues := map[string]any{"managementAgentId": "ocid1.managementagent.oc1..synthetic"}
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	resource.Status.OsokStatus.Ocid = "ocid1.managementagent.oc1..synthetic"
	resource.Status.Key = resourceID
	resource.Spec.Name = "synthetic-data-source"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..synthetic"
	resource.Spec.Type = "PROMETHEUS_EMITTER"
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "", map[string]any{"compartmentId": "ocid1.compartment.oc1..synthetic", "key": resourceID, "managementAgentId": "ocid1.managementagent.oc1..synthetic", "name": "synthetic-data-source", "resourceId": resourceID, "state": "ACTIVE", "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "type": "PROMETHEUS_EMITTER"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "managementagent", Resource: "DataSource", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "datasource_synthetic_read.yaml"), Host: "https://management-agent.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200202", Metadata: metadata, Bindings: map[string]string{"compartment": "ocid1.compartment.oc1..synthetic"}, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := managementagentsdk.ManagementAgentClient{BaseClient: session.BaseClient()}
	client := newDataSourceRuntimeClient(sdkClient, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked DataSource response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic DataSource cassette: %w", err))
	}
}
