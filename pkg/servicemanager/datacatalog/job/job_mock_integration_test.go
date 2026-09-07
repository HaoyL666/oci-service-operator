/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package job

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

func TestMockIntegrationJobCompositeCRUD(t *testing.T) {
	t.Parallel()
	resource := &datacatalogv1beta1.Job{}
	ocimock.InitializeResource(resource, "mock-job")
	resource.Spec = ocimock.MustJSONFixture[datacatalogv1beta1.JobSpec](t, `{
  "catalogId": "<ocid:1>",
  "displayName": "job create",
  "jobDefinitionKey": "job-definition-key",
  "description": "create"
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{"description":"updated"}`)
	createdState := ocimock.MustOCIResponseFixture[datacatalogsdk.Job](t, `{
  "catalogId": "<ocid:1>",
  "displayName": "job create",
  "jobDefinitionKey": "job-definition-key",
  "description": "create",
  "key": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	updatedState := ocimock.MustOCIResponseFixture[datacatalogsdk.Job](t, `{
  "catalogId": "<ocid:1>",
  "displayName": "job create",
  "jobDefinitionKey": "job-definition-key",
  "description": "updated",
  "key": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[datacatalogsdk.Job, struct{}, struct{}]{
		CollectionPath: "/20190325/catalogs/<ocid:1>/jobs", ItemPath: "/20190325/catalogs/<ocid:1>/jobs/resource-key",
		Operations:   []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreatedState: &createdState, UpdatedState: &updatedState, ListShape: ocimock.ListShapeItems,
		RequireCreateRead: true, RequireUpdateRead: true, RequireDeleteRead: true, DeleteEndsNotFound: true,
		CreateStatus: 201, UpdateStatus: 200, DeleteStatus: 204, NotFoundCode: "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error { return validateJobBodyField(request, "displayName", "job create") },
		ValidateUpdateRaw: func(request ocimock.Request) error { return validateJobBodyField(request, "description", "updated") },
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
	manager := &JobServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newJobRuntimeHooks(manager, sdkClient)
	client := wrapJobGeneratedClient(hooks, defaultJobServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.Job](buildJobGeneratedRuntimeConfig(manager, hooks))})
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*datacatalogv1beta1.Job]{
		Resource: resource, Client: client, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *datacatalogv1beta1.Job) error {
			if current.Status.Description != resource.Spec.Description || current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Job status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *datacatalogv1beta1.Job) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *datacatalogv1beta1.Job) error {
			if current.Status.Description != current.Spec.Description {
				return fmt.Errorf("updated Job status = %+v", current.Status)
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

func validateJobBodyField(request ocimock.Request, field string, want string) error {
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
