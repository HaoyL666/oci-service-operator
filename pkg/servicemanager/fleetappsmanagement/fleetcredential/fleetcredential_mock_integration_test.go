/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package fleetcredential

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	fleetappsmanagementsdk "github.com/oracle/oci-go-sdk/v65/fleetappsmanagement"
	fleetappsmanagementv1beta1 "github.com/oracle/oci-service-operator/api/fleetappsmanagement/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMockIntegrationFleetCredentialCompositeCRUD(t *testing.T) {
	t.Parallel()
	resource := &fleetappsmanagementv1beta1.FleetCredential{}
	ocimock.InitializeResource(resource, "mock-fleetcredential")
	resource.Spec = ocimock.MustJSONFixture[fleetappsmanagementv1beta1.FleetCredentialSpec](t, `{
  "fleetId": "<ocid:1>",
  "displayName": "credential create",
  "entitySpecifics": {
    "credentialLevel": "FLEET"
  },
  "user": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-user"
  },
  "password": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-password"
  }
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{"displayName":"credential updated"}`)
	createdState := ocimock.MustOCIResponseFixture[fleetappsmanagementsdk.FleetCredential](t, `{
  "displayName": "credential create",
  "entitySpecifics": {
    "credentialLevel": "FLEET"
  },
  "user": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-user"
  },
  "password": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-password"
  },
  "id": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	updatedState := ocimock.MustOCIResponseFixture[fleetappsmanagementsdk.FleetCredential](t, `{
  "displayName": "credential updated",
  "entitySpecifics": {
    "credentialLevel": "FLEET"
  },
  "user": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-user"
  },
  "password": {
    "credentialType": "PLAIN_TEXT",
    "value": "mock-password"
  },
  "id": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[fleetappsmanagementsdk.FleetCredential, struct{}, struct{}]{
		CollectionPath: "/20250228/fleets/<ocid:1>/fleetCredentials", ItemPath: "/20250228/fleets/<ocid:1>/fleetCredentials/resource-key",
		Operations:   []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreatedState: &createdState, UpdatedState: &updatedState, ListShape: ocimock.ListShapeItems,
		RequireCreateRead: true, RequireUpdateRead: true, RequireDeleteRead: true, DeleteEndsNotFound: true,
		CreateStatus: 201, UpdateStatus: 200, DeleteStatus: 204, NotFoundCode: "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error {
			return validateFleetCredentialBodyField(request, "displayName", "credential create")
		},
		ValidateUpdateRaw: func(request ocimock.Request) error {
			return validateFleetCredentialBodyField(request, "displayName", "credential updated")
		},
		CreateHeaders: http.Header{"Opc-Work-Request-Id": []string{"wr-create"}},
		UpdateHeaders: http.Header{"Opc-Work-Request-Id": []string{"wr-update"}},
		DeleteHeaders: http.Header{"Opc-Work-Request-Id": []string{"wr-delete"}},
		AdditionalRoutes: []ocimock.Route{
			{Name: "create-work-request", Method: http.MethodGet, Path: "/20250228/workRequests/wr-create", MinimumCalls: 1, Respond: func(ocimock.Request) (ocimock.Response, error) {
				return fleetCredentialWorkRequestResponse("wr-create", fleetappsmanagementsdk.OperationTypeCreateCredential, fleetappsmanagementsdk.ActionTypeCreated)
			}},
			{Name: "update-work-request", Method: http.MethodGet, Path: "/20250228/workRequests/wr-update", MinimumCalls: 1, Respond: func(ocimock.Request) (ocimock.Response, error) {
				return fleetCredentialWorkRequestResponse("wr-update", fleetappsmanagementsdk.OperationTypeUpdateCredential, fleetappsmanagementsdk.ActionTypeUpdated)
			}},
			{Name: "delete-work-request", Method: http.MethodGet, Path: "/20250228/workRequests/wr-delete", MinimumCalls: 1, Respond: func(ocimock.Request) (ocimock.Response, error) {
				return fleetCredentialWorkRequestResponse("wr-delete", fleetappsmanagementsdk.OperationTypeDeleteCredential, fleetappsmanagementsdk.ActionTypeDeleted)
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://fleetappsmanagement.mock.invalid", BasePath: "20250228", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sdkClient := fleetappsmanagementsdk.FleetAppsManagementClient{BaseClient: session.BaseClient()}
	workRequestClient := fleetappsmanagementsdk.FleetAppsManagementWorkRequestClient{BaseClient: session.BaseClient()}
	client := newFleetCredentialServiceClientWithOCIClients(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}, sdkClient, workRequestClient)
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*fleetappsmanagementv1beta1.FleetCredential]{
		Resource: resource, Client: client, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *fleetappsmanagementv1beta1.FleetCredential) error {
			if current.Status.DisplayName != resource.Spec.DisplayName || current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created FleetCredential status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *fleetappsmanagementv1beta1.FleetCredential) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *fleetappsmanagementv1beta1.FleetCredential) error {
			if current.Status.DisplayName != current.Spec.DisplayName {
				return fmt.Errorf("updated FleetCredential status = %+v", current.Status)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func fleetCredentialWorkRequestResponse(id string, operation fleetappsmanagementsdk.OperationTypeEnum, action fleetappsmanagementsdk.ActionTypeEnum) (ocimock.Response, error) {
	complete := float32(100)
	return ocimock.JSONResponse(http.StatusOK, fleetappsmanagementsdk.WorkRequest{
		OperationType: operation, Status: fleetappsmanagementsdk.OperationStatusSucceeded, Id: common.String(id),
		CompartmentId: common.String("<ocid:2>"), PercentComplete: &complete,
		Resources: []fleetappsmanagementsdk.WorkRequestResource{{EntityType: common.String("FleetCredential"), ActionType: action, Identifier: common.String("resource-key")}},
	})
}

func validateFleetCredentialBodyField(request ocimock.Request, field string, want string) error {
	var body map[string]any
	if err := ocimock.DecodeJSONRequest(request, &body); err != nil {
		return err
	}
	value, ok := body[field]
	if !ok {
		return fmt.Errorf("%s %s body is missing %s", request.Method, request.URL.Path, field)
	}
	if want != "" {
		got, ok := value.(string)
		if !ok || got != want {
			return fmt.Errorf("%s %s body[%s] = %#v, want %q", request.Method, request.URL.Path, field, value, want)
		}
	}
	return nil
}
