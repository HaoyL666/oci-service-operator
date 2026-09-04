/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package serviceconnector

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	schsdk "github.com/oracle/oci-go-sdk/v65/sch"
	schv1beta1 "github.com/oracle/oci-service-operator/api/sch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticServiceConnectorCreateReadDelete(t *testing.T) {
	resource := newServiceConnectorResource()
	resource.Spec.DisplayName = "osok-replay-service-connector"
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.serviceconnector.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	createPolls := 0
	deleted := false
	responder := func(request *http.Request) (ocireplay.SyntheticResponse, error) {
		path := request.URL.Path
		switch {
		case request.Method == http.MethodPost && strings.HasSuffix(path, "/serviceConnectors"):
			return ocireplay.SyntheticResponse{StatusCode: http.StatusAccepted, Headers: http.Header{"Opc-Work-Request-Id": []string{"ocid1.serviceconnectorworkrequest.oc1..create"}}}, nil
		case request.Method == http.MethodGet && strings.Contains(path, "/workRequests/"):
			if deleted {
				return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: serviceConnectorSyntheticWorkRequest("DELETE_SERVICE_CONNECTOR", "SUCCEEDED", "DELETED")}, nil
			}
			createPolls++
			if createPolls == 1 {
				return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: serviceConnectorSyntheticWorkRequest("CREATE_SERVICE_CONNECTOR", "IN_PROGRESS", "IN_PROGRESS")}, nil
			}
			return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: serviceConnectorSyntheticWorkRequest("CREATE_SERVICE_CONNECTOR", "SUCCEEDED", "CREATED")}, nil
		case request.Method == http.MethodDelete && strings.Contains(path, "/serviceConnectors/"):
			deleted = true
			return ocireplay.SyntheticResponse{StatusCode: http.StatusAccepted, Headers: http.Header{"Opc-Work-Request-Id": []string{"ocid1.serviceconnectorworkrequest.oc1..delete"}}}, nil
		case request.Method == http.MethodGet && strings.Contains(path, "/serviceConnectors/") && deleted:
			return ocireplay.SyntheticResponse{StatusCode: http.StatusNotFound, Body: `{"code":"NotFound","message":"resource deleted"}`}, nil
		case request.Method == http.MethodGet && strings.Contains(path, "/serviceConnectors/"):
			return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: createdBody}, nil
		case request.Method == http.MethodGet && strings.HasSuffix(path, "/serviceConnectors"):
			if deleted {
				return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: `{"items":[]}`}, nil
			}
			return ocireplay.SyntheticResponse{StatusCode: http.StatusOK, Body: `{"items":[` + createdBody + `]}`}, nil
		default:
			return ocireplay.SyntheticResponse{}, fmt.Errorf("unsupported ServiceConnector synthetic request %s %s", request.Method, path)
		}
	}
	metadata := ocireplay.Metadata{Service: "sch", Resource: "ServiceConnector", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "serviceconnector_synthetic_crud.yaml"), Host: "https://sch.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200909", Metadata: metadata, Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := schsdk.ServiceConnectorClient{BaseClient: session.BaseClient()}
	client := newServiceConnectorServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*schv1beta1.ServiceConnector]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *schv1beta1.ServiceConnector) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *schv1beta1.ServiceConnector) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created ServiceConnector status = %+v", current.Status)
			}
			return nil
		},
	})
}

func serviceConnectorSyntheticWorkRequest(operation string, status string, action string) string {
	return fmt.Sprintf(`{"id":"ocid1.serviceconnectorworkrequest.oc1..synthetic","operationType":%q,"status":%q,"percentComplete":100,"resources":[{"actionType":%q,"entityType":"serviceConnector","entityUri":"/serviceConnectors/ocid1.serviceconnector.oc1..synthetic","identifier":"ocid1.serviceconnector.oc1..synthetic"}]}`, operation, status, action)
}
