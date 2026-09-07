/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package entity

import (
	"context"
	"fmt"
	"testing"

	datacatalogsdk "github.com/oracle/oci-go-sdk/v65/datacatalog"
	datacatalogv1beta1 "github.com/oracle/oci-service-operator/api/datacatalog/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMockIntegrationEntityCompositeCRUD(t *testing.T) {
	t.Parallel()
	resource := &datacatalogv1beta1.Entity{}
	ocimock.InitializeResource(resource, "mock-entity")
	resource.Spec = ocimock.MustJSONFixture[datacatalogv1beta1.EntitySpec](t, `{
  "catalogId": "<ocid:1>",
  "dataAssetKey": "asset-key",
  "displayName": "entity create",
  "timeExternal": "2026-01-02T03:04:05Z",
  "description": "create"
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{"description":"updated"}`)
	createdState := ocimock.MustOCIResponseFixture[datacatalogsdk.Entity](t, `{
  "dataAssetKey": "asset-key",
  "displayName": "entity create",
  "timeExternal": "2026-01-02T03:04:05Z",
  "description": "create",
  "key": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	updatedState := ocimock.MustOCIResponseFixture[datacatalogsdk.Entity](t, `{
  "dataAssetKey": "asset-key",
  "displayName": "entity create",
  "timeExternal": "2026-01-02T03:04:05Z",
  "description": "updated",
  "key": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[datacatalogsdk.Entity, struct{}, struct{}]{
		CollectionPath: "/20190325/catalogs/<ocid:1>/dataAssets/asset-key/entities", ItemPath: "/20190325/catalogs/<ocid:1>/dataAssets/asset-key/entities/resource-key",
		Operations:   []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreatedState: &createdState, UpdatedState: &updatedState, ListShape: ocimock.ListShapeItems,
		RequireCreateRead: true, RequireUpdateRead: true, RequireDeleteRead: true, DeleteEndsNotFound: true,
		CreateStatus: 201, UpdateStatus: 200, DeleteStatus: 204, NotFoundCode: "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error {
			return validateEntityBodyField(request, "displayName", "entity create")
		},
		ValidateUpdateRaw: func(request ocimock.Request) error { return validateEntityBodyField(request, "description", "updated") },
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://datacatalog.mock.invalid", BasePath: "20190325", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sdkClient := datacatalogsdk.DataCatalogClient{BaseClient: session.BaseClient()}
	manager := &EntityServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newEntityRuntimeHooks(manager, sdkClient)
	client := wrapEntityGeneratedClient(hooks, defaultEntityServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.Entity](buildEntityGeneratedRuntimeConfig(manager, hooks))})
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*datacatalogv1beta1.Entity]{
		Resource: resource, Client: client, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *datacatalogv1beta1.Entity) error {
			if current.Status.Description != resource.Spec.Description || current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Entity status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *datacatalogv1beta1.Entity) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *datacatalogv1beta1.Entity) error {
			if current.Status.Description != current.Spec.Description {
				return fmt.Errorf("updated Entity status = %+v", current.Status)
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

func validateEntityBodyField(request ocimock.Request, field string, want string) error {
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
