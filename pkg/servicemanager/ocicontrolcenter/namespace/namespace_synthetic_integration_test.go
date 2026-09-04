/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package namespace

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	ocicontrolcenterv1beta1 "github.com/oracle/oci-service-operator/api/ocicontrolcenter/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticNamespaceReadsExisting(t *testing.T) {
	resource := &ocicontrolcenterv1beta1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "synthetic-namespace"}}
	metadata := ocireplay.Metadata{Service: "ocicontrolcenter", Resource: "Namespace", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "namespace_synthetic_read.yaml"), Host: "https://control-center.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230515", Metadata: metadata,
		Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: `{"items":[{"namespaceName":"synthetic-namespace"}]}`}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := ocicontrolcentersdk.OccMetricsClient{BaseClient: session.BaseClient()}
	provider := common.NewRawConfigurationProvider("ocid1.tenancy.oc1..synthetic", "user", "us-ashburn-1", "fingerprint", "private-key", nil)
	client := newNamespaceRuntimeClientForTest(provider, sdkClient.ListNamespaces)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.NamespaceName != "synthetic-namespace" {
		t.Fatalf("Namespace response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
