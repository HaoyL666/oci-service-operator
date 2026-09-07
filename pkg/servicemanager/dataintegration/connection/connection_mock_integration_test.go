/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package connection

import (
	"context"
	"fmt"
	"testing"

	dataintegrationsdk "github.com/oracle/oci-go-sdk/v65/dataintegration"
	dataintegrationv1beta1 "github.com/oracle/oci-service-operator/api/dataintegration/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMockIntegrationConnectionCompositeCRUD(t *testing.T) {
	t.Parallel()
	resource := &dataintegrationv1beta1.Connection{}
	ocimock.InitializeResource(resource, "mock-connection")
	resource.Spec = ocimock.MustJSONFixture[dataintegrationv1beta1.ConnectionSpec](t, `{
  "workspaceId": "<ocid:1>",
  "dataAssetKey": "data-asset-key",
  "name": "connection create",
  "identifier": "CONNECTION_CREATE",
  "objectVersion": 1,
  "modelType": "REST_NO_AUTH_CONNECTION",
  "description": "create"
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{"description":"updated"}`)
	createdState := ocimock.MustOCIResponseFixture[dataintegrationsdk.ConnectionFromRestNoAuth](t, `{
  "dataAssetKey": "data-asset-key",
  "name": "connection create",
  "identifier": "CONNECTION_CREATE",
  "objectVersion": 1,
  "modelType": "REST_NO_AUTH_CONNECTION",
  "description": "create",
  "key": "resource-key"
}`)
	updatedState := ocimock.MustOCIResponseFixture[dataintegrationsdk.ConnectionFromRestNoAuth](t, `{
  "dataAssetKey": "data-asset-key",
  "name": "connection create",
  "identifier": "CONNECTION_CREATE",
  "objectVersion": 1,
  "modelType": "REST_NO_AUTH_CONNECTION",
  "description": "updated",
  "key": "resource-key"
}`)
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[dataintegrationsdk.ConnectionFromRestNoAuth, struct{}, struct{}]{
		CollectionPath: "/20200430/workspaces/<ocid:1>/connections", ItemPath: "/20200430/workspaces/<ocid:1>/connections/resource-key",
		Operations:   []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreatedState: &createdState, UpdatedState: &updatedState, ListShape: ocimock.ListShapeItems,
		RequireCreateRead: true, RequireUpdateRead: true, RequireDeleteRead: true, DeleteEndsNotFound: true,
		CreateStatus: 201, UpdateStatus: 200, DeleteStatus: 204, NotFoundCode: "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error {
			return validateConnectionBodyField(request, "name", "connection create")
		},
		ValidateUpdateRaw: func(request ocimock.Request) error {
			return validateConnectionBodyField(request, "description", "updated")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://dataintegration.mock.invalid", BasePath: "20200430", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sdkClient := dataintegrationsdk.DataIntegrationClient{BaseClient: session.BaseClient()}
	manager := &ConnectionServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newConnectionRuntimeHooks(manager, sdkClient)
	client := wrapConnectionGeneratedClient(hooks, defaultConnectionServiceClient{ServiceClient: generatedruntime.NewServiceClient[*dataintegrationv1beta1.Connection](buildConnectionGeneratedRuntimeConfig(manager, hooks))})
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*dataintegrationv1beta1.Connection]{
		Resource: resource, Client: client, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *dataintegrationv1beta1.Connection) error {
			if current.Status.Description != resource.Spec.Description || current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Connection status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dataintegrationv1beta1.Connection) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *dataintegrationv1beta1.Connection) error {
			if current.Status.Description != current.Spec.Description {
				return fmt.Errorf("updated Connection status = %+v", current.Status)
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

func validateConnectionBodyField(request ocimock.Request, field string, want string) error {
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
