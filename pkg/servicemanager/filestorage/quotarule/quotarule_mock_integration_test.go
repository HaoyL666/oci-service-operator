/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package quotarule

import (
	"context"
	"fmt"
	"testing"

	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMockIntegrationQuotaRuleCompositeCRUD(t *testing.T) {
	t.Parallel()
	resource := &filestoragev1beta1.QuotaRule{}
	ocimock.InitializeResource(resource, "mock-quotarule")
	resource.Spec = ocimock.MustJSONFixture[filestoragev1beta1.QuotaRuleSpec](t, `{
  "fileSystemId": "<ocid:1>",
  "principalType": "INDIVIDUAL_USER",
  "isHardQuota": true,
  "quotaLimitInGigabytes": 10,
  "principalId": 1000,
  "displayName": "quota create"
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{"displayName":"quota updated"}`)
	createdState := ocimock.MustOCIResponseFixture[filestoragesdk.QuotaRule](t, `{
  "fileSystemId": "<ocid:1>",
  "principalType": "INDIVIDUAL_USER",
  "isHardQuota": true,
  "quotaLimitInGigabytes": 10,
  "principalId": 1000,
  "displayName": "quota create",
  "id": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	updatedState := ocimock.MustOCIResponseFixture[filestoragesdk.QuotaRule](t, `{
  "fileSystemId": "<ocid:1>",
  "principalType": "INDIVIDUAL_USER",
  "isHardQuota": true,
  "quotaLimitInGigabytes": 10,
  "principalId": 1000,
  "displayName": "quota updated",
  "id": "resource-key",
  "lifecycleState": "ACTIVE"
}`)
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[filestoragesdk.QuotaRule, struct{}, struct{}]{
		CollectionPath: "/20171215/fileSystems/<ocid:1>/quotaRules", ItemPath: "/20171215/fileSystems/<ocid:1>/quotaRules/resource-key",
		Operations:   []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreatedState: &createdState, UpdatedState: &updatedState, ListShape: ocimock.ListShapeArray,
		RequireCreateRead: true, RequireUpdateRead: true, RequireDeleteRead: true, DeleteEndsNotFound: true,
		CreateStatus: 201, UpdateStatus: 200, DeleteStatus: 204, NotFoundCode: "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error {
			return validateQuotaRuleBodyField(request, "displayName", "quota create")
		},
		ValidateUpdateRaw: func(request ocimock.Request) error {
			return validateQuotaRuleBodyField(request, "displayName", "quota updated")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://filestorage.mock.invalid", BasePath: "20171215", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	sdkClient := filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}
	manager := &QuotaRuleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newQuotaRuleRuntimeHooks(manager, sdkClient)
	client := wrapQuotaRuleGeneratedClient(hooks, defaultQuotaRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.QuotaRule](buildQuotaRuleGeneratedRuntimeConfig(manager, hooks))})
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*filestoragev1beta1.QuotaRule]{
		Resource: resource, Client: client, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *filestoragev1beta1.QuotaRule) error {
			if current.Status.DisplayName != resource.Spec.DisplayName || current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created QuotaRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *filestoragev1beta1.QuotaRule) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *filestoragev1beta1.QuotaRule) error {
			if current.Status.DisplayName != current.Spec.DisplayName {
				return fmt.Errorf("updated QuotaRule status = %+v", current.Status)
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

func validateQuotaRuleBodyField(request ocimock.Request, field string, want string) error {
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
