/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package workitem

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticWorkItemReadsTracked(t *testing.T) {
	workRequestID := "ocid1.workrequest.oc1..synthetic"
	workItemID := "work-item-synthetic"
	resource := &jmsutilsv1beta1.WorkItem{ObjectMeta: metav1.ObjectMeta{
		Name: "synthetic-work-item",
		Annotations: map[string]string{
			workItemWorkRequestIDAnnotation: workRequestID,
			workItemIDAnnotation:            workItemID,
		},
	}}
	body := `{"items":[{"details":{"kind":"BASIC"},"id":"work-item-synthetic","retryCount":0,"status":"SUCCEEDED","workRequestId":"ocid1.workrequest.oc1..synthetic"}]}`
	metadata := ocireplay.Metadata{Service: "jmsutils", Resource: "WorkItem", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "workitem_synthetic_read.yaml"), Host: "https://javamanagement-utils.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250521", Metadata: metadata,
		Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: body}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := jmsutilssdk.JmsUtilsClient{BaseClient: session.BaseClient()}
	client := newWorkItemServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Id != workItemID || resource.Status.Status != "SUCCEEDED" {
		t.Fatalf("WorkItem response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
