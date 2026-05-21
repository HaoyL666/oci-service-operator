/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package datasource

import (
	"context"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	generativeaiagentsdk "github.com/oracle/oci-go-sdk/v65/generativeaiagent"
	generativeaiagentv1beta1 "github.com/oracle/oci-service-operator/api/generativeaiagent/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type fakeDataSourceOCIClient struct {
	createFn      func(context.Context, generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error)
	getFn         func(context.Context, generativeaiagentsdk.GetDataSourceRequest) (generativeaiagentsdk.GetDataSourceResponse, error)
	listFn        func(context.Context, generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error)
	updateFn      func(context.Context, generativeaiagentsdk.UpdateDataSourceRequest) (generativeaiagentsdk.UpdateDataSourceResponse, error)
	deleteFn      func(context.Context, generativeaiagentsdk.DeleteDataSourceRequest) (generativeaiagentsdk.DeleteDataSourceResponse, error)
	workRequestFn func(context.Context, generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error)
}

func (f *fakeDataSourceOCIClient) CreateDataSource(
	ctx context.Context,
	req generativeaiagentsdk.CreateDataSourceRequest,
) (generativeaiagentsdk.CreateDataSourceResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return generativeaiagentsdk.CreateDataSourceResponse{}, nil
}

func (f *fakeDataSourceOCIClient) GetDataSource(
	ctx context.Context,
	req generativeaiagentsdk.GetDataSourceRequest,
) (generativeaiagentsdk.GetDataSourceResponse, error) {
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return generativeaiagentsdk.GetDataSourceResponse{}, errortest.NewServiceError(404, "NotFound", "missing")
}

func (f *fakeDataSourceOCIClient) ListDataSources(
	ctx context.Context,
	req generativeaiagentsdk.ListDataSourcesRequest,
) (generativeaiagentsdk.ListDataSourcesResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return generativeaiagentsdk.ListDataSourcesResponse{}, nil
}

func (f *fakeDataSourceOCIClient) UpdateDataSource(
	ctx context.Context,
	req generativeaiagentsdk.UpdateDataSourceRequest,
) (generativeaiagentsdk.UpdateDataSourceResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return generativeaiagentsdk.UpdateDataSourceResponse{}, nil
}

func (f *fakeDataSourceOCIClient) DeleteDataSource(
	ctx context.Context,
	req generativeaiagentsdk.DeleteDataSourceRequest,
) (generativeaiagentsdk.DeleteDataSourceResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return generativeaiagentsdk.DeleteDataSourceResponse{}, nil
}

func (f *fakeDataSourceOCIClient) GetWorkRequest(
	ctx context.Context,
	req generativeaiagentsdk.GetWorkRequestRequest,
) (generativeaiagentsdk.GetWorkRequestResponse, error) {
	if f.workRequestFn != nil {
		return f.workRequestFn(ctx, req)
	}
	return generativeaiagentsdk.GetWorkRequestResponse{}, nil
}

func TestReviewedDataSourceRuntimeSemanticsEncodesWorkRequestContract(t *testing.T) {
	t.Parallel()

	got := reviewedDataSourceRuntimeSemantics()
	if got == nil {
		t.Fatal("reviewedDataSourceRuntimeSemantics() = nil")
	}

	if got.FormalService != "generativeaiagent" {
		t.Fatalf("FormalService = %q, want generativeaiagent", got.FormalService)
	}
	if got.FormalSlug != "datasource" {
		t.Fatalf("FormalSlug = %q, want datasource", got.FormalSlug)
	}
	if got.Async == nil {
		t.Fatal("Async = nil, want workrequest semantics")
	}
	if got.Async.Strategy != "workrequest" {
		t.Fatalf("Async.Strategy = %q, want workrequest", got.Async.Strategy)
	}
	if got.Async.Runtime != "generatedruntime" {
		t.Fatalf("Async.Runtime = %q, want generatedruntime", got.Async.Runtime)
	}
	if got.Async.WorkRequest == nil {
		t.Fatal("Async.WorkRequest = nil")
	}
	assertDataSourceStringSliceEqual(t, "Async.WorkRequest.Phases", got.Async.WorkRequest.Phases, []string{"create", "update", "delete"})
	assertDataSourceStringSliceEqual(t, "Lifecycle.ProvisioningStates", got.Lifecycle.ProvisioningStates, []string{"CREATING"})
	assertDataSourceStringSliceEqual(t, "Lifecycle.UpdatingStates", got.Lifecycle.UpdatingStates, []string{"UPDATING"})
	assertDataSourceStringSliceEqual(t, "Lifecycle.ActiveStates", got.Lifecycle.ActiveStates, []string{"ACTIVE", "INACTIVE"})
	assertDataSourceStringSliceEqual(t, "Delete.PendingStates", got.Delete.PendingStates, []string{"DELETING"})
	assertDataSourceStringSliceEqual(t, "Delete.TerminalStates", got.Delete.TerminalStates, []string{"DELETED"})
	assertDataSourceStringSliceEqual(t, "List.MatchFields", got.List.MatchFields, []string{"compartmentId", "knowledgeBaseId", "displayName"})
	assertDataSourceStringSliceEqual(t, "Mutation.Mutable", got.Mutation.Mutable, []string{"dataSourceConfig", "definedTags", "description", "displayName", "freeformTags", "metadata"})
	assertDataSourceStringSliceEqual(t, "Mutation.ForceNew", got.Mutation.ForceNew, []string{"compartmentId", "knowledgeBaseId"})
	if got.FinalizerPolicy != "retain-until-confirmed-delete" {
		t.Fatalf("FinalizerPolicy = %q, want retain-until-confirmed-delete", got.FinalizerPolicy)
	}
	if got.Delete.Policy != "required" {
		t.Fatalf("Delete.Policy = %q, want required", got.Delete.Policy)
	}
	if got.CreateFollowUp.Strategy != "GetWorkRequest -> GetDataSource" {
		t.Fatalf("CreateFollowUp.Strategy = %q, want workrequest-backed create", got.CreateFollowUp.Strategy)
	}
	if got.UpdateFollowUp.Strategy != "GetWorkRequest -> GetDataSource" {
		t.Fatalf("UpdateFollowUp.Strategy = %q, want workrequest-backed update", got.UpdateFollowUp.Strategy)
	}
	if got.DeleteFollowUp.Strategy != "GetWorkRequest -> GetDataSource/ListDataSources confirm-delete" {
		t.Fatalf("DeleteFollowUp.Strategy = %q, want workrequest-backed confirm-delete", got.DeleteFollowUp.Strategy)
	}
	if len(got.AuxiliaryOperations) != 0 {
		t.Fatalf("AuxiliaryOperations = %#v, want none for published runtime", got.AuxiliaryOperations)
	}
}

func TestBuildDataSourceCreateDetailsPreservesConcreteConfigAndFalseBool(t *testing.T) {
	t.Parallel()

	resource := makeDataSourceResource()

	details, err := buildDataSourceCreateDetails(context.Background(), resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildDataSourceCreateDetails() error = %v", err)
	}

	requireDataSourceStringPtr(t, "details.compartmentId", details.CompartmentId, resource.Spec.CompartmentId)
	requireDataSourceStringPtr(t, "details.knowledgeBaseId", details.KnowledgeBaseId, resource.Spec.KnowledgeBaseId)
	requireDataSourceStringPtr(t, "details.displayName", details.DisplayName, resource.Spec.DisplayName)
	requireDataSourceStringPtr(t, "details.description", details.Description, resource.Spec.Description)
	if !maps.Equal(details.Metadata, resource.Spec.Metadata) {
		t.Fatalf("details.Metadata = %#v, want %#v", details.Metadata, resource.Spec.Metadata)
	}
	requireDataSourceObjectStorageConfig(t, "details.dataSourceConfig", details.DataSourceConfig, resource.Spec.DataSourceConfig)
}

func TestBuildDataSourceCreateDetailsSupportsJSONDataConfig(t *testing.T) {
	t.Parallel()

	resource := makeDataSourceResource()
	resource.Spec.DataSourceConfig = generativeaiagentv1beta1.DataSourceConfig{
		JsonData: `{"dataSourceConfigType":"OCI_OBJECT_STORAGE","shouldEnableMultiModality":false,"objectStoragePrefixes":[{"namespaceName":"namespace-json","bucketName":"bucket-json","prefix":"json-prefix/"}]}`,
	}

	details, err := buildDataSourceCreateDetails(context.Background(), resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildDataSourceCreateDetails() error = %v", err)
	}

	requireDataSourceObjectStorageConfig(t, "details.dataSourceConfig", details.DataSourceConfig, generativeaiagentv1beta1.DataSourceConfig{
		DataSourceConfigType:      "OCI_OBJECT_STORAGE",
		ShouldEnableMultiModality: false,
		ObjectStoragePrefixes: []generativeaiagentv1beta1.DataSourceConfigObjectStoragePrefix{
			{
				NamespaceName: "namespace-json",
				BucketName:    "bucket-json",
				Prefix:        "json-prefix/",
			},
		},
	})
}

func TestBuildDataSourceUpdateBodyPreservesClearsAndConfigChanges(t *testing.T) {
	t.Parallel()

	currentResource := makeDataSourceResource()
	currentResource.Spec.Description = "current description"
	currentResource.Spec.Metadata = map[string]string{
		"source": "current",
	}
	currentResource.Spec.DataSourceConfig.ShouldEnableMultiModality = true
	currentResource.Spec.DataSourceConfig.ObjectStoragePrefixes = []generativeaiagentv1beta1.DataSourceConfigObjectStoragePrefix{
		{
			NamespaceName: "namespace-current",
			BucketName:    "bucket-current",
			Prefix:        "current/",
		},
	}

	desired := makeDataSourceResource()
	desired.Spec.Description = ""
	desired.Spec.Metadata = map[string]string{}
	desired.Spec.FreeformTags = map[string]string{}
	desired.Spec.DefinedTags = map[string]shared.MapValue{}

	body, updateNeeded, err := buildDataSourceUpdateBody(
		context.Background(),
		desired,
		desired.Namespace,
		generativeaiagentsdk.GetDataSourceResponse{
			DataSource: makeSDKDataSource("ocid1.datasource.oc1..existing", currentResource, generativeaiagentsdk.DataSourceLifecycleStateActive),
		},
	)
	if err != nil {
		t.Fatalf("buildDataSourceUpdateBody() error = %v", err)
	}
	if !updateNeeded {
		t.Fatal("buildDataSourceUpdateBody() updateNeeded = false, want true")
	}

	requireDataSourceStringPtr(t, "details.description", body.Description, "")
	if len(body.Metadata) != 0 {
		t.Fatalf("details.Metadata = %#v, want empty map for clear", body.Metadata)
	}
	if len(body.FreeformTags) != 0 {
		t.Fatalf("details.FreeformTags = %#v, want empty map for clear", body.FreeformTags)
	}
	if len(body.DefinedTags) != 0 {
		t.Fatalf("details.DefinedTags = %#v, want empty map for clear", body.DefinedTags)
	}
	requireDataSourceObjectStorageConfig(t, "details.dataSourceConfig", body.DataSourceConfig, desired.Spec.DataSourceConfig)
}

func TestDataSourceCreateOrUpdateSkipsReuseWhenDisplayNameMissing(t *testing.T) {
	t.Parallel()

	resource := makeDataSourceResource()
	resource.Spec.DisplayName = ""

	const (
		createdID     = "ocid1.datasource.oc1..created"
		workRequestID = "wr-datasource-create-empty-name"
	)

	listCalls := 0
	createCalls := 0

	client := newTestDataSourceClient(&fakeDataSourceOCIClient{
		listFn: func(_ context.Context, _ generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error) {
			listCalls++
			return generativeaiagentsdk.ListDataSourcesResponse{}, nil
		},
		createFn: func(_ context.Context, req generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error) {
			createCalls++
			requireDataSourceStringPtr(t, "create compartmentId", req.CreateDataSourceDetails.CompartmentId, resource.Spec.CompartmentId)
			requireDataSourceStringPtr(t, "create knowledgeBaseId", req.CreateDataSourceDetails.KnowledgeBaseId, resource.Spec.KnowledgeBaseId)
			if req.CreateDataSourceDetails.DisplayName != nil {
				t.Fatalf("create displayName = %v, want nil when spec.displayName is empty", req.CreateDataSourceDetails.DisplayName)
			}
			return generativeaiagentsdk.CreateDataSourceResponse{
				DataSource:       makeSDKDataSource(createdID, resource, generativeaiagentsdk.DataSourceLifecycleStateCreating),
				OpcWorkRequestId: common.String(workRequestID),
				OpcRequestId:     common.String("opc-create-empty-name"),
			}, nil
		},
		workRequestFn: func(_ context.Context, req generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error) {
			requireDataSourceStringPtr(t, "workRequestId", req.WorkRequestId, workRequestID)
			return generativeaiagentsdk.GetWorkRequestResponse{
				WorkRequest: makeDataSourceWorkRequest(
					workRequestID,
					generativeaiagentsdk.OperationTypeCreateDataSource,
					generativeaiagentsdk.OperationStatusInProgress,
					generativeaiagentsdk.ActionTypeInProgress,
					createdID,
				),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful pending create", response)
	}
	if listCalls != 0 {
		t.Fatalf("ListDataSources() calls = %d, want 0 when displayName is empty", listCalls)
	}
	if createCalls != 1 {
		t.Fatalf("CreateDataSource() calls = %d, want 1", createCalls)
	}
	requireDataSourceAsyncCurrent(t, resource, shared.OSOKAsyncPhaseCreate, workRequestID, shared.OSOKAsyncClassPending)
	if got := resource.Status.Id; got != createdID {
		t.Fatalf("status.id = %q, want %q from create response body", got, createdID)
	}
}

func TestDataSourceCreateOrUpdateRejectsAmbiguousDisplayNameReuse(t *testing.T) {
	t.Parallel()

	resource := makeDataSourceResource()
	createCalls := 0

	client := newTestDataSourceClient(&fakeDataSourceOCIClient{
		listFn: func(_ context.Context, req generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error) {
			requireDataSourceStringPtr(t, "list compartmentId", req.CompartmentId, resource.Spec.CompartmentId)
			requireDataSourceStringPtr(t, "list knowledgeBaseId", req.KnowledgeBaseId, resource.Spec.KnowledgeBaseId)
			requireDataSourceStringPtr(t, "list displayName", req.DisplayName, resource.Spec.DisplayName)
			return generativeaiagentsdk.ListDataSourcesResponse{
				DataSourceCollection: generativeaiagentsdk.DataSourceCollection{
					Items: []generativeaiagentsdk.DataSourceSummary{
						makeSDKDataSourceSummary("ocid1.datasource.oc1..first", resource, generativeaiagentsdk.DataSourceLifecycleStateActive),
						makeSDKDataSourceSummary("ocid1.datasource.oc1..second", resource, generativeaiagentsdk.DataSourceLifecycleStateInactive),
					},
				},
			}, nil
		},
		createFn: func(_ context.Context, _ generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error) {
			createCalls++
			return generativeaiagentsdk.CreateDataSourceResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want ambiguous list match failure")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful result", response)
	}
	if !strings.Contains(err.Error(), "multiple matching resources") {
		t.Fatalf("CreateOrUpdate() error = %v, want duplicate match failure", err)
	}
	if createCalls != 0 {
		t.Fatalf("CreateDataSource() calls = %d, want 0 on ambiguous reuse", createCalls)
	}
}

func TestDataSourceServiceClientCreatesAndResumesWorkRequest(t *testing.T) {
	t.Parallel()

	const (
		createdID     = "ocid1.datasource.oc1..created"
		workRequestID = "wr-datasource-create"
	)

	resource := makeDataSourceResource()
	workRequests := map[string]generativeaiagentsdk.WorkRequest{
		workRequestID: makeDataSourceWorkRequest(
			workRequestID,
			generativeaiagentsdk.OperationTypeCreateDataSource,
			generativeaiagentsdk.OperationStatusInProgress,
			generativeaiagentsdk.ActionTypeInProgress,
			createdID,
		),
	}

	var createRequest generativeaiagentsdk.CreateDataSourceRequest
	getCalls := 0

	client := newTestDataSourceClient(&fakeDataSourceOCIClient{
		listFn: func(_ context.Context, req generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error) {
			requireDataSourceStringPtr(t, "list compartmentId", req.CompartmentId, resource.Spec.CompartmentId)
			requireDataSourceStringPtr(t, "list knowledgeBaseId", req.KnowledgeBaseId, resource.Spec.KnowledgeBaseId)
			requireDataSourceStringPtr(t, "list displayName", req.DisplayName, resource.Spec.DisplayName)
			return generativeaiagentsdk.ListDataSourcesResponse{}, nil
		},
		createFn: func(_ context.Context, req generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error) {
			createRequest = req
			return generativeaiagentsdk.CreateDataSourceResponse{
				DataSource:       makeSDKDataSource(createdID, resource, generativeaiagentsdk.DataSourceLifecycleStateCreating),
				OpcWorkRequestId: common.String(workRequestID),
				OpcRequestId:     common.String("opc-create-datasource"),
			}, nil
		},
		workRequestFn: func(_ context.Context, req generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error) {
			requireDataSourceStringPtr(t, "workRequestId", req.WorkRequestId, workRequestID)
			return generativeaiagentsdk.GetWorkRequestResponse{WorkRequest: workRequests[workRequestID]}, nil
		},
		getFn: func(_ context.Context, req generativeaiagentsdk.GetDataSourceRequest) (generativeaiagentsdk.GetDataSourceResponse, error) {
			getCalls++
			requireDataSourceStringPtr(t, "get dataSourceId", req.DataSourceId, createdID)
			return generativeaiagentsdk.GetDataSourceResponse{
				DataSource: makeSDKDataSource(createdID, resource, generativeaiagentsdk.DataSourceLifecycleStateActive),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful pending create", response)
	}
	requireDataSourceStringPtr(t, "create compartmentId", createRequest.CreateDataSourceDetails.CompartmentId, resource.Spec.CompartmentId)
	requireDataSourceStringPtr(t, "create knowledgeBaseId", createRequest.CreateDataSourceDetails.KnowledgeBaseId, resource.Spec.KnowledgeBaseId)
	requireDataSourceStringPtr(t, "create displayName", createRequest.CreateDataSourceDetails.DisplayName, resource.Spec.DisplayName)
	requireDataSourceObjectStorageConfig(t, "create dataSourceConfig", createRequest.CreateDataSourceDetails.DataSourceConfig, resource.Spec.DataSourceConfig)
	if getCalls != 0 {
		t.Fatalf("GetDataSource() calls = %d, want 0 while work request is pending", getCalls)
	}
	requireDataSourceAsyncCurrent(t, resource, shared.OSOKAsyncPhaseCreate, workRequestID, shared.OSOKAsyncClassPending)
	if got := resource.Status.Id; got != createdID {
		t.Fatalf("status.id = %q, want %q from create response body", got, createdID)
	}
	if got := resource.Status.LifecycleState; got != string(generativeaiagentsdk.DataSourceLifecycleStateCreating) {
		t.Fatalf("status.lifecycleState = %q, want CREATING", got)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-create-datasource" {
		t.Fatalf("status.opcRequestId = %q, want opc-create-datasource", got)
	}

	workRequests[workRequestID] = makeDataSourceWorkRequest(
		workRequestID,
		generativeaiagentsdk.OperationTypeCreateDataSource,
		generativeaiagentsdk.OperationStatusSucceeded,
		generativeaiagentsdk.ActionTypeCreated,
		createdID,
	)

	response, err = client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() after work request success error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() after work request success response = %#v, want converged success", response)
	}
	if getCalls != 1 {
		t.Fatalf("GetDataSource() calls = %d, want 1 follow-up read", getCalls)
	}
	if got := resource.Status.Id; got != createdID {
		t.Fatalf("status.id = %q, want %q", got, createdID)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != createdID {
		t.Fatalf("status.ocid = %q, want %q", got, createdID)
	}
	if got := resource.Status.LifecycleState; got != string(generativeaiagentsdk.DataSourceLifecycleStateActive) {
		t.Fatalf("status.lifecycleState = %q, want ACTIVE", got)
	}
	requireDataSourceStatusConfig(t, "status.dataSourceConfig", resource.Status.DataSourceConfig, resource.Spec.DataSourceConfig)
	if !maps.Equal(resource.Status.Metadata, resource.Spec.Metadata) {
		t.Fatalf("status.metadata = %#v, want %#v", resource.Status.Metadata, resource.Spec.Metadata)
	}
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.async.current = %#v, want cleared", resource.Status.OsokStatus.Async.Current)
	}
}

func newTestDataSourceClient(client *fakeDataSourceOCIClient) DataSourceServiceClient {
	if client == nil {
		client = &fakeDataSourceOCIClient{}
	}
	return newDataSourceServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")},
		client,
	)
}

func makeDataSourceResource() *generativeaiagentv1beta1.DataSource {
	return &generativeaiagentv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-source-sample",
			Namespace: "default",
		},
		Spec: generativeaiagentv1beta1.DataSourceSpec{
			KnowledgeBaseId: "ocid1.knowledgebase.oc1..datasourceexample",
			CompartmentId:   "ocid1.compartment.oc1..datasourceexample",
			DisplayName:     "data-source-sample",
			Description:     "data-source description",
			DataSourceConfig: generativeaiagentv1beta1.DataSourceConfig{
				DataSourceConfigType:      "OCI_OBJECT_STORAGE",
				ShouldEnableMultiModality: false,
				ObjectStoragePrefixes: []generativeaiagentv1beta1.DataSourceConfigObjectStoragePrefix{
					{
						NamespaceName: "namespace-a",
						BucketName:    "bucket-a",
						Prefix:        "documents/",
					},
				},
			},
			Metadata: map[string]string{
				"source": "objectstorage",
			},
			FreeformTags: map[string]string{
				"environment": "dev",
			},
			DefinedTags: map[string]shared.MapValue{
				"Operations": {
					"CostCenter": "42",
				},
			},
		},
	}
}

func makeSDKDataSource(
	id string,
	resource *generativeaiagentv1beta1.DataSource,
	state generativeaiagentsdk.DataSourceLifecycleStateEnum,
) generativeaiagentsdk.DataSource {
	now := common.SDKTime{Time: time.Unix(0, 0).UTC()}
	return generativeaiagentsdk.DataSource{
		Id:               common.String(id),
		DisplayName:      common.String(resource.Spec.DisplayName),
		CompartmentId:    common.String(resource.Spec.CompartmentId),
		KnowledgeBaseId:  common.String(resource.Spec.KnowledgeBaseId),
		DataSourceConfig: makeSDKDataSourceConfig(resource.Spec.DataSourceConfig),
		TimeCreated:      &now,
		LifecycleState:   state,
		FreeformTags:     maps.Clone(resource.Spec.FreeformTags),
		DefinedTags:      sdkDefinedTags(resource.Spec.DefinedTags),
		Description:      common.String(resource.Spec.Description),
		Metadata:         maps.Clone(resource.Spec.Metadata),
		TimeUpdated:      &now,
		LifecycleDetails: common.String("lifecycle detail"),
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {
				"free-tier-retained": "true",
			},
		},
	}
}

func makeSDKDataSourceSummary(
	id string,
	resource *generativeaiagentv1beta1.DataSource,
	state generativeaiagentsdk.DataSourceLifecycleStateEnum,
) generativeaiagentsdk.DataSourceSummary {
	now := common.SDKTime{Time: time.Unix(0, 0).UTC()}
	return generativeaiagentsdk.DataSourceSummary{
		Id:               common.String(id),
		DisplayName:      common.String(resource.Spec.DisplayName),
		KnowledgeBaseId:  common.String(resource.Spec.KnowledgeBaseId),
		CompartmentId:    common.String(resource.Spec.CompartmentId),
		TimeCreated:      &now,
		LifecycleState:   state,
		FreeformTags:     maps.Clone(resource.Spec.FreeformTags),
		DefinedTags:      sdkDefinedTags(resource.Spec.DefinedTags),
		Description:      common.String(resource.Spec.Description),
		TimeUpdated:      &now,
		LifecycleDetails: common.String("lifecycle detail"),
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {
				"free-tier-retained": "true",
			},
		},
	}
}

func makeSDKDataSourceConfig(
	spec generativeaiagentv1beta1.DataSourceConfig,
) generativeaiagentsdk.DataSourceConfig {
	switch strings.TrimSpace(spec.DataSourceConfigType) {
	case "OCI_OBJECT_STORAGE":
		prefixes := make([]generativeaiagentsdk.ObjectStoragePrefix, 0, len(spec.ObjectStoragePrefixes))
		for _, prefix := range spec.ObjectStoragePrefixes {
			projected := generativeaiagentsdk.ObjectStoragePrefix{
				NamespaceName: common.String(prefix.NamespaceName),
				BucketName:    common.String(prefix.BucketName),
			}
			if prefix.Prefix != "" {
				projected.Prefix = common.String(prefix.Prefix)
			}
			prefixes = append(prefixes, projected)
		}
		return generativeaiagentsdk.OciObjectStorageDataSourceConfig{
			ShouldEnableMultiModality: common.Bool(spec.ShouldEnableMultiModality),
			ObjectStoragePrefixes:     prefixes,
		}
	default:
		return nil
	}
}

func makeDataSourceWorkRequest(
	id string,
	operation generativeaiagentsdk.OperationTypeEnum,
	status generativeaiagentsdk.OperationStatusEnum,
	action generativeaiagentsdk.ActionTypeEnum,
	resourceID string,
) generativeaiagentsdk.WorkRequest {
	now := common.SDKTime{Time: time.Unix(0, 0).UTC()}
	percentComplete := float32(50)
	return generativeaiagentsdk.WorkRequest{
		OperationType:   operation,
		Status:          status,
		Id:              common.String(id),
		CompartmentId:   common.String("ocid1.compartment.oc1..datasourceexample"),
		Resources:       []generativeaiagentsdk.WorkRequestResource{{EntityType: common.String("DataSource"), ActionType: action, Identifier: common.String(resourceID)}},
		PercentComplete: &percentComplete,
		TimeAccepted:    &now,
	}
}

func sdkDefinedTags(input map[string]shared.MapValue) map[string]map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(input))
	for namespace, values := range input {
		converted := make(map[string]interface{}, len(values))
		for key, value := range values {
			converted[key] = value
		}
		out[namespace] = converted
	}
	return out
}

func assertDataSourceStringSliceEqual(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func requireDataSourceStringPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %q", name, got, want)
	}
}

func requireDataSourceObjectStorageConfig(
	t *testing.T,
	name string,
	got generativeaiagentsdk.DataSourceConfig,
	want generativeaiagentv1beta1.DataSourceConfig,
) {
	t.Helper()
	config, ok := got.(generativeaiagentsdk.OciObjectStorageDataSourceConfig)
	if !ok {
		t.Fatalf("%s type = %T, want OciObjectStorageDataSourceConfig", name, got)
	}
	if config.ShouldEnableMultiModality == nil || *config.ShouldEnableMultiModality != want.ShouldEnableMultiModality {
		t.Fatalf("%s.ShouldEnableMultiModality = %v, want %t", name, config.ShouldEnableMultiModality, want.ShouldEnableMultiModality)
	}
	if len(config.ObjectStoragePrefixes) != len(want.ObjectStoragePrefixes) {
		t.Fatalf("%s.ObjectStoragePrefixes length = %d, want %d", name, len(config.ObjectStoragePrefixes), len(want.ObjectStoragePrefixes))
	}
	for i, prefix := range want.ObjectStoragePrefixes {
		if config.ObjectStoragePrefixes[i].NamespaceName == nil || *config.ObjectStoragePrefixes[i].NamespaceName != prefix.NamespaceName {
			t.Fatalf("%s.ObjectStoragePrefixes[%d].NamespaceName = %v, want %q", name, i, config.ObjectStoragePrefixes[i].NamespaceName, prefix.NamespaceName)
		}
		if config.ObjectStoragePrefixes[i].BucketName == nil || *config.ObjectStoragePrefixes[i].BucketName != prefix.BucketName {
			t.Fatalf("%s.ObjectStoragePrefixes[%d].BucketName = %v, want %q", name, i, config.ObjectStoragePrefixes[i].BucketName, prefix.BucketName)
		}
		wantPrefix := prefix.Prefix
		gotPrefix := ""
		if config.ObjectStoragePrefixes[i].Prefix != nil {
			gotPrefix = *config.ObjectStoragePrefixes[i].Prefix
		}
		if gotPrefix != wantPrefix {
			t.Fatalf("%s.ObjectStoragePrefixes[%d].Prefix = %q, want %q", name, i, gotPrefix, wantPrefix)
		}
	}
}

func requireDataSourceStatusConfig(
	t *testing.T,
	name string,
	got generativeaiagentv1beta1.DataSourceConfig,
	want generativeaiagentv1beta1.DataSourceConfig,
) {
	t.Helper()
	if got.DataSourceConfigType != want.DataSourceConfigType {
		t.Fatalf("%s.DataSourceConfigType = %q, want %q", name, got.DataSourceConfigType, want.DataSourceConfigType)
	}
	if got.ShouldEnableMultiModality != want.ShouldEnableMultiModality {
		t.Fatalf("%s.ShouldEnableMultiModality = %t, want %t", name, got.ShouldEnableMultiModality, want.ShouldEnableMultiModality)
	}
	if !reflect.DeepEqual(got.ObjectStoragePrefixes, want.ObjectStoragePrefixes) {
		t.Fatalf("%s.ObjectStoragePrefixes = %#v, want %#v", name, got.ObjectStoragePrefixes, want.ObjectStoragePrefixes)
	}
}

func requireDataSourceAsyncCurrent(
	t *testing.T,
	resource *generativeaiagentv1beta1.DataSource,
	phase shared.OSOKAsyncPhase,
	workRequestID string,
	class shared.OSOKAsyncNormalizedClass,
) {
	t.Helper()
	current := resource.Status.OsokStatus.Async.Current
	if current == nil {
		t.Fatal("status.async.current = nil, want tracked work request")
	}
	if current.Source != shared.OSOKAsyncSourceWorkRequest {
		t.Fatalf("status.async.current.source = %q, want %q", current.Source, shared.OSOKAsyncSourceWorkRequest)
	}
	if current.Phase != phase {
		t.Fatalf("status.async.current.phase = %q, want %q", current.Phase, phase)
	}
	if current.WorkRequestID != workRequestID {
		t.Fatalf("status.async.current.workRequestId = %q, want %q", current.WorkRequestID, workRequestID)
	}
	if current.NormalizedClass != class {
		t.Fatalf("status.async.current.normalizedClass = %q, want %q", current.NormalizedClass, class)
	}
}
