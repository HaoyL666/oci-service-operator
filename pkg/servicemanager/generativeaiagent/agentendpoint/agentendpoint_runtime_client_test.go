/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package agentendpoint

import (
	"context"
	"maps"
	"reflect"
	"slices"
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

type fakeAgentEndpointOCIClient struct {
	createFn      func(context.Context, generativeaiagentsdk.CreateAgentEndpointRequest) (generativeaiagentsdk.CreateAgentEndpointResponse, error)
	getFn         func(context.Context, generativeaiagentsdk.GetAgentEndpointRequest) (generativeaiagentsdk.GetAgentEndpointResponse, error)
	listFn        func(context.Context, generativeaiagentsdk.ListAgentEndpointsRequest) (generativeaiagentsdk.ListAgentEndpointsResponse, error)
	updateFn      func(context.Context, generativeaiagentsdk.UpdateAgentEndpointRequest) (generativeaiagentsdk.UpdateAgentEndpointResponse, error)
	deleteFn      func(context.Context, generativeaiagentsdk.DeleteAgentEndpointRequest) (generativeaiagentsdk.DeleteAgentEndpointResponse, error)
	workRequestFn func(context.Context, generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error)
}

func (f *fakeAgentEndpointOCIClient) CreateAgentEndpoint(
	ctx context.Context,
	req generativeaiagentsdk.CreateAgentEndpointRequest,
) (generativeaiagentsdk.CreateAgentEndpointResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return generativeaiagentsdk.CreateAgentEndpointResponse{}, nil
}

func (f *fakeAgentEndpointOCIClient) GetAgentEndpoint(
	ctx context.Context,
	req generativeaiagentsdk.GetAgentEndpointRequest,
) (generativeaiagentsdk.GetAgentEndpointResponse, error) {
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return generativeaiagentsdk.GetAgentEndpointResponse{}, errortest.NewServiceError(404, "NotFound", "missing")
}

func (f *fakeAgentEndpointOCIClient) ListAgentEndpoints(
	ctx context.Context,
	req generativeaiagentsdk.ListAgentEndpointsRequest,
) (generativeaiagentsdk.ListAgentEndpointsResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return generativeaiagentsdk.ListAgentEndpointsResponse{}, nil
}

func (f *fakeAgentEndpointOCIClient) UpdateAgentEndpoint(
	ctx context.Context,
	req generativeaiagentsdk.UpdateAgentEndpointRequest,
) (generativeaiagentsdk.UpdateAgentEndpointResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return generativeaiagentsdk.UpdateAgentEndpointResponse{}, nil
}

func (f *fakeAgentEndpointOCIClient) DeleteAgentEndpoint(
	ctx context.Context,
	req generativeaiagentsdk.DeleteAgentEndpointRequest,
) (generativeaiagentsdk.DeleteAgentEndpointResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return generativeaiagentsdk.DeleteAgentEndpointResponse{}, nil
}

func (f *fakeAgentEndpointOCIClient) GetWorkRequest(
	ctx context.Context,
	req generativeaiagentsdk.GetWorkRequestRequest,
) (generativeaiagentsdk.GetWorkRequestResponse, error) {
	if f.workRequestFn != nil {
		return f.workRequestFn(ctx, req)
	}
	return generativeaiagentsdk.GetWorkRequestResponse{}, nil
}

func TestReviewedAgentEndpointRuntimeSemanticsEncodesWorkRequestContract(t *testing.T) {
	t.Parallel()

	got := reviewedAgentEndpointRuntimeSemantics()
	if got == nil {
		t.Fatal("reviewedAgentEndpointRuntimeSemantics() = nil")
	}

	if got.FormalService != "generativeaiagent" {
		t.Fatalf("FormalService = %q, want generativeaiagent", got.FormalService)
	}
	if got.FormalSlug != "agentendpoint" {
		t.Fatalf("FormalSlug = %q, want agentendpoint", got.FormalSlug)
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
	assertAgentEndpointStringSliceEqual(t, "Async.WorkRequest.Phases", got.Async.WorkRequest.Phases, []string{"create", "update", "delete"})
	assertAgentEndpointStringSliceEqual(t, "Lifecycle.ProvisioningStates", got.Lifecycle.ProvisioningStates, []string{"CREATING"})
	assertAgentEndpointStringSliceEqual(t, "Lifecycle.UpdatingStates", got.Lifecycle.UpdatingStates, []string{"UPDATING"})
	assertAgentEndpointStringSliceEqual(t, "Lifecycle.ActiveStates", got.Lifecycle.ActiveStates, []string{"ACTIVE"})
	assertAgentEndpointStringSliceEqual(t, "Delete.PendingStates", got.Delete.PendingStates, []string{"DELETING"})
	assertAgentEndpointStringSliceEqual(t, "Delete.TerminalStates", got.Delete.TerminalStates, []string{"DELETED"})
	assertAgentEndpointStringSliceEqual(t, "List.MatchFields", got.List.MatchFields, []string{"compartmentId", "agentId", "displayName"})
	assertAgentEndpointStringSliceEqual(t, "Mutation.Mutable", got.Mutation.Mutable, []string{
		"contentModerationConfig",
		"definedTags",
		"description",
		"displayName",
		"freeformTags",
		"guardrailConfig",
		"humanInputConfig",
		"metadata",
		"outputConfig",
		"provisionedCapacityConfig",
		"sessionConfig",
		"shouldEnableCitation",
		"shouldEnableMultiLanguage",
		"shouldEnableTrace",
	})
	assertAgentEndpointStringSliceEqual(t, "Mutation.ForceNew", got.Mutation.ForceNew, []string{"agentId", "compartmentId", "shouldEnableSession"})
	if got.FinalizerPolicy != "retain-until-confirmed-delete" {
		t.Fatalf("FinalizerPolicy = %q, want retain-until-confirmed-delete", got.FinalizerPolicy)
	}
	if got.Delete.Policy != "required" {
		t.Fatalf("Delete.Policy = %q, want required", got.Delete.Policy)
	}
	if got.CreateFollowUp.Strategy != "GetWorkRequest -> GetAgentEndpoint" {
		t.Fatalf("CreateFollowUp.Strategy = %q, want workrequest-backed create", got.CreateFollowUp.Strategy)
	}
	if got.UpdateFollowUp.Strategy != "GetWorkRequest -> GetAgentEndpoint" {
		t.Fatalf("UpdateFollowUp.Strategy = %q, want workrequest-backed update", got.UpdateFollowUp.Strategy)
	}
	if got.DeleteFollowUp.Strategy != "GetWorkRequest -> GetAgentEndpoint/ListAgentEndpoints confirm-delete" {
		t.Fatalf("DeleteFollowUp.Strategy = %q, want workrequest-backed confirm-delete", got.DeleteFollowUp.Strategy)
	}
	if len(got.AuxiliaryOperations) != 0 {
		t.Fatalf("AuxiliaryOperations = %#v, want none for published runtime", got.AuxiliaryOperations)
	}
}

func TestBuildAgentEndpointCreateDetailsPreservesConcreteOutputLocationAndFalseBooleans(t *testing.T) {
	t.Parallel()

	resource := makeAgentEndpointResource()

	details, err := buildAgentEndpointCreateDetails(context.Background(), resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildAgentEndpointCreateDetails() error = %v", err)
	}

	requireAgentEndpointStringPtr(t, "details.agentId", details.AgentId, resource.Spec.AgentId)
	requireAgentEndpointStringPtr(t, "details.compartmentId", details.CompartmentId, resource.Spec.CompartmentId)
	requireAgentEndpointStringPtr(t, "details.displayName", details.DisplayName, resource.Spec.DisplayName)
	requireAgentEndpointStringPtr(t, "details.description", details.Description, resource.Spec.Description)
	requireAgentEndpointOptionalBoolPtr(t, "details.shouldEnableTrace", details.ShouldEnableTrace, false)
	requireAgentEndpointOptionalBoolPtr(t, "details.shouldEnableSession", details.ShouldEnableSession, true)
	if details.HumanInputConfig == nil {
		t.Fatal("details.HumanInputConfig = nil, want populated config")
	}
	requireAgentEndpointOptionalBoolPtr(t, "details.humanInputConfig.shouldEnableHumanInput", details.HumanInputConfig.ShouldEnableHumanInput, false)
	if details.OutputConfig == nil {
		t.Fatal("details.OutputConfig = nil, want populated config")
	}
	if details.OutputConfig.RetentionPeriodInMinutes == nil || *details.OutputConfig.RetentionPeriodInMinutes != resource.Spec.OutputConfig.RetentionPeriodInMinutes {
		t.Fatalf("details.outputConfig.retentionPeriodInMinutes = %v, want %d", details.OutputConfig.RetentionPeriodInMinutes, resource.Spec.OutputConfig.RetentionPeriodInMinutes)
	}
	requireAgentEndpointOutputLocationPrefix(t, "details.outputConfig.outputLocation", details.OutputConfig.OutputLocation, resource.Spec.OutputConfig.OutputLocation)
}

func TestBuildAgentEndpointUpdateBodyPreservesClearsAndFalseValues(t *testing.T) {
	t.Parallel()

	currentResource := makeAgentEndpointResource()
	currentResource.Spec.Description = "current description"
	currentResource.Spec.Metadata = map[string]string{
		"legacy": "true",
	}
	currentResource.Spec.ContentModerationConfig = generativeaiagentv1beta1.AgentEndpointContentModerationConfig{
		ShouldEnableOnInput:  true,
		ShouldEnableOnOutput: true,
	}
	currentResource.Spec.HumanInputConfig.ShouldEnableHumanInput = true
	currentResource.Spec.OutputConfig.OutputLocation.Prefix = "results/current/"
	currentResource.Spec.ShouldEnableTrace = true
	currentResource.Spec.ShouldEnableCitation = true
	currentResource.Spec.ShouldEnableMultiLanguage = true
	currentResource.Spec.SessionConfig.IdleTimeoutInSeconds = 900
	currentResource.Spec.ProvisionedCapacityConfig.PlatformRuntimeConfig.Version = "2024.05.01"
	currentResource.Spec.ProvisionedCapacityConfig.ToolRuntimeConfigs[0].Version = "1.0"

	desired := makeAgentEndpointResource()
	desired.Spec.Description = ""
	desired.Spec.Metadata = map[string]string{}
	desired.Spec.ContentModerationConfig = generativeaiagentv1beta1.AgentEndpointContentModerationConfig{
		ShouldEnableOnInput:  false,
		ShouldEnableOnOutput: false,
	}
	desired.Spec.HumanInputConfig.ShouldEnableHumanInput = false
	desired.Spec.OutputConfig.OutputLocation.Prefix = "results/updated/"
	desired.Spec.ShouldEnableTrace = false
	desired.Spec.ShouldEnableCitation = false
	desired.Spec.ShouldEnableMultiLanguage = false
	desired.Spec.SessionConfig.IdleTimeoutInSeconds = 300
	desired.Spec.ProvisionedCapacityConfig.PlatformRuntimeConfig.Version = "2024.06.01"
	desired.Spec.ProvisionedCapacityConfig.ToolRuntimeConfigs[0].Version = "2.0"
	desired.Spec.FreeformTags = map[string]string{}
	desired.Spec.DefinedTags = map[string]shared.MapValue{}

	body, updateNeeded, err := buildAgentEndpointUpdateBody(
		context.Background(),
		desired,
		desired.Namespace,
		generativeaiagentsdk.GetAgentEndpointResponse{
			AgentEndpoint: makeSDKAgentEndpoint(t, "ocid1.agentendpoint.oc1..existing", currentResource, generativeaiagentsdk.AgentEndpointLifecycleStateActive),
		},
	)
	if err != nil {
		t.Fatalf("buildAgentEndpointUpdateBody() error = %v", err)
	}
	if !updateNeeded {
		t.Fatal("buildAgentEndpointUpdateBody() updateNeeded = false, want true")
	}

	requireAgentEndpointStringPtr(t, "details.description", body.Description, "")
	if len(body.Metadata) != 0 {
		t.Fatalf("details.Metadata = %#v, want empty map for clear", body.Metadata)
	}
	if body.ContentModerationConfig == nil {
		t.Fatal("details.ContentModerationConfig = nil, want populated config")
	}
	requireAgentEndpointOptionalBoolPtr(t, "details.contentModerationConfig.shouldEnableOnInput", body.ContentModerationConfig.ShouldEnableOnInput, false)
	requireAgentEndpointOptionalBoolPtr(t, "details.contentModerationConfig.shouldEnableOnOutput", body.ContentModerationConfig.ShouldEnableOnOutput, false)
	if body.HumanInputConfig == nil {
		t.Fatal("details.HumanInputConfig = nil, want populated config")
	}
	requireAgentEndpointOptionalBoolPtr(t, "details.humanInputConfig.shouldEnableHumanInput", body.HumanInputConfig.ShouldEnableHumanInput, false)
	if body.OutputConfig == nil {
		t.Fatal("details.OutputConfig = nil, want updated output config")
	}
	requireAgentEndpointOutputLocationPrefix(t, "details.outputConfig.outputLocation", body.OutputConfig.OutputLocation, desired.Spec.OutputConfig.OutputLocation)
	requireAgentEndpointOptionalBoolPtr(t, "details.shouldEnableTrace", body.ShouldEnableTrace, false)
	requireAgentEndpointOptionalBoolPtr(t, "details.shouldEnableCitation", body.ShouldEnableCitation, false)
	requireAgentEndpointOptionalBoolPtr(t, "details.shouldEnableMultiLanguage", body.ShouldEnableMultiLanguage, false)
	if body.SessionConfig == nil || body.SessionConfig.IdleTimeoutInSeconds == nil || *body.SessionConfig.IdleTimeoutInSeconds != desired.Spec.SessionConfig.IdleTimeoutInSeconds {
		t.Fatalf("details.SessionConfig = %#v, want idleTimeoutInSeconds=%d", body.SessionConfig, desired.Spec.SessionConfig.IdleTimeoutInSeconds)
	}
	if body.ProvisionedCapacityConfig == nil || body.ProvisionedCapacityConfig.PlatformRuntimeConfig == nil {
		t.Fatalf("details.ProvisionedCapacityConfig = %#v, want populated config", body.ProvisionedCapacityConfig)
	}
	requireAgentEndpointStringPtr(t, "details.provisionedCapacityConfig.platformRuntimeConfig.version", body.ProvisionedCapacityConfig.PlatformRuntimeConfig.Version, desired.Spec.ProvisionedCapacityConfig.PlatformRuntimeConfig.Version)
	if len(body.FreeformTags) != 0 {
		t.Fatalf("details.FreeformTags = %#v, want empty map for clear", body.FreeformTags)
	}
	if len(body.DefinedTags) != 0 {
		t.Fatalf("details.DefinedTags = %#v, want empty map for clear", body.DefinedTags)
	}
}

func TestValidateAgentEndpointCreateOnlyDriftRejectsNilSessionReadbackWhenSpecRequiresTrue(t *testing.T) {
	t.Parallel()

	resource := makeAgentEndpointResource()
	current := makeSDKAgentEndpoint(t, "ocid1.agentendpoint.oc1..existing", resource, generativeaiagentsdk.AgentEndpointLifecycleStateActive)
	current.ShouldEnableSession = nil

	err := validateAgentEndpointCreateOnlyDriftForResponse(resource, current)
	if err == nil {
		t.Fatal("validateAgentEndpointCreateOnlyDriftForResponse() error = nil, want shouldEnableSession drift failure")
	}
	if !strings.Contains(err.Error(), "shouldEnableSession") {
		t.Fatalf("validateAgentEndpointCreateOnlyDriftForResponse() error = %v, want shouldEnableSession detail", err)
	}
}

func TestAgentEndpointCreateOrUpdateSkipsReuseWhenDisplayNameMissing(t *testing.T) {
	t.Parallel()

	resource := makeAgentEndpointResource()
	resource.Spec.DisplayName = ""

	const (
		createdID     = "ocid1.agentendpoint.oc1..created"
		workRequestID = "wr-agentendpoint-create-empty-name"
	)

	listCalls := 0
	createCalls := 0

	client := newTestAgentEndpointClient(&fakeAgentEndpointOCIClient{
		listFn: func(_ context.Context, _ generativeaiagentsdk.ListAgentEndpointsRequest) (generativeaiagentsdk.ListAgentEndpointsResponse, error) {
			listCalls++
			return generativeaiagentsdk.ListAgentEndpointsResponse{}, nil
		},
		createFn: func(_ context.Context, req generativeaiagentsdk.CreateAgentEndpointRequest) (generativeaiagentsdk.CreateAgentEndpointResponse, error) {
			createCalls++
			requireAgentEndpointStringPtr(t, "create compartmentId", req.CreateAgentEndpointDetails.CompartmentId, resource.Spec.CompartmentId)
			requireAgentEndpointStringPtr(t, "create agentId", req.CreateAgentEndpointDetails.AgentId, resource.Spec.AgentId)
			if req.CreateAgentEndpointDetails.DisplayName != nil {
				t.Fatalf("create displayName = %v, want nil when spec.displayName is empty", req.CreateAgentEndpointDetails.DisplayName)
			}
			return generativeaiagentsdk.CreateAgentEndpointResponse{
				AgentEndpoint:    makeSDKAgentEndpoint(t, createdID, resource, generativeaiagentsdk.AgentEndpointLifecycleStateCreating),
				OpcWorkRequestId: common.String(workRequestID),
				OpcRequestId:     common.String("opc-create-empty-name"),
			}, nil
		},
		workRequestFn: func(_ context.Context, req generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error) {
			requireAgentEndpointStringPtr(t, "workRequestId", req.WorkRequestId, workRequestID)
			return generativeaiagentsdk.GetWorkRequestResponse{
				WorkRequest: makeAgentEndpointWorkRequest(
					workRequestID,
					generativeaiagentsdk.OperationTypeCreateAgentEndpoint,
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
		t.Fatalf("ListAgentEndpoints() calls = %d, want 0 when displayName is empty", listCalls)
	}
	if createCalls != 1 {
		t.Fatalf("CreateAgentEndpoint() calls = %d, want 1", createCalls)
	}
	requireAgentEndpointAsyncCurrent(t, resource, shared.OSOKAsyncPhaseCreate, workRequestID, shared.OSOKAsyncClassPending)
}

func TestAgentEndpointCreateOrUpdateReusesUniqueExactListMatch(t *testing.T) {
	t.Parallel()

	resource := makeAgentEndpointResource()

	const existingID = "ocid1.agentendpoint.oc1..existing"

	listCalls := 0
	getCalls := 0

	client := newTestAgentEndpointClient(&fakeAgentEndpointOCIClient{
		listFn: func(_ context.Context, req generativeaiagentsdk.ListAgentEndpointsRequest) (generativeaiagentsdk.ListAgentEndpointsResponse, error) {
			listCalls++
			requireAgentEndpointStringPtr(t, "list compartmentId", req.CompartmentId, resource.Spec.CompartmentId)
			requireAgentEndpointStringPtr(t, "list agentId", req.AgentId, resource.Spec.AgentId)
			requireAgentEndpointStringPtr(t, "list displayName", req.DisplayName, resource.Spec.DisplayName)
			return generativeaiagentsdk.ListAgentEndpointsResponse{
				AgentEndpointCollection: generativeaiagentsdk.AgentEndpointCollection{
					Items: []generativeaiagentsdk.AgentEndpointSummary{
						makeSDKAgentEndpointSummary(t, existingID, resource, generativeaiagentsdk.AgentEndpointLifecycleStateActive),
					},
				},
			}, nil
		},
		getFn: func(_ context.Context, req generativeaiagentsdk.GetAgentEndpointRequest) (generativeaiagentsdk.GetAgentEndpointResponse, error) {
			getCalls++
			requireAgentEndpointStringPtr(t, "get agentEndpointId", req.AgentEndpointId, existingID)
			return generativeaiagentsdk.GetAgentEndpointResponse{
				AgentEndpoint: makeSDKAgentEndpoint(t, existingID, resource, generativeaiagentsdk.AgentEndpointLifecycleStateActive),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want converged reused resource", response)
	}
	if listCalls != 1 {
		t.Fatalf("ListAgentEndpoints() calls = %d, want 1", listCalls)
	}
	if getCalls != 1 {
		t.Fatalf("GetAgentEndpoint() calls = %d, want 1 forced live read after reuse", getCalls)
	}
	if got := resource.Status.Id; got != existingID {
		t.Fatalf("status.id = %q, want %q", got, existingID)
	}
	if got := resource.Status.LifecycleState; got != string(generativeaiagentsdk.AgentEndpointLifecycleStateActive) {
		t.Fatalf("status.lifecycleState = %q, want ACTIVE", got)
	}
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.async.current = %#v, want cleared", resource.Status.OsokStatus.Async.Current)
	}
}

func newTestAgentEndpointClient(client *fakeAgentEndpointOCIClient) AgentEndpointServiceClient {
	if client == nil {
		client = &fakeAgentEndpointOCIClient{}
	}
	return newAgentEndpointServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")},
		client,
	)
}

func makeAgentEndpointResource() *generativeaiagentv1beta1.AgentEndpoint {
	return &generativeaiagentv1beta1.AgentEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentendpoint-sample",
			Namespace: "default",
		},
		Spec: generativeaiagentv1beta1.AgentEndpointSpec{
			AgentId:       "ocid1.agent.oc1..example",
			CompartmentId: "ocid1.compartment.oc1..example",
			DisplayName:   "agentendpoint-sample",
			Description:   "agent endpoint description",
			ContentModerationConfig: generativeaiagentv1beta1.AgentEndpointContentModerationConfig{
				ShouldEnableOnInput:  true,
				ShouldEnableOnOutput: false,
			},
			GuardrailConfig: generativeaiagentv1beta1.AgentEndpointGuardrailConfig{
				ContentModerationConfig: generativeaiagentv1beta1.AgentEndpointGuardrailConfigContentModerationConfig{
					InputGuardrailMode:  "BLOCK",
					OutputGuardrailMode: "REDACT",
				},
				PromptInjectionConfig: generativeaiagentv1beta1.AgentEndpointGuardrailConfigPromptInjectionConfig{
					InputGuardrailMode: "BLOCK",
				},
				PersonallyIdentifiableInformationConfig: generativeaiagentv1beta1.AgentEndpointGuardrailConfigPersonallyIdentifiableInformationConfig{
					InputGuardrailMode:  "REDACT",
					OutputGuardrailMode: "REDACT",
				},
			},
			Metadata: map[string]string{
				"environment": "dev",
			},
			HumanInputConfig: generativeaiagentv1beta1.AgentEndpointHumanInputConfig{
				ShouldEnableHumanInput: false,
			},
			OutputConfig: generativeaiagentv1beta1.AgentEndpointOutputConfig{
				OutputLocation: generativeaiagentv1beta1.AgentEndpointOutputConfigOutputLocation{
					OutputLocationType: "OBJECT_STORAGE_PREFIX",
					NamespaceName:      "namespace",
					BucketName:         "bucket",
					Prefix:             "results/",
				},
				RetentionPeriodInMinutes: 60,
			},
			ShouldEnableTrace:         false,
			ShouldEnableCitation:      true,
			ShouldEnableSession:       true,
			ShouldEnableMultiLanguage: true,
			SessionConfig: generativeaiagentv1beta1.AgentEndpointSessionConfig{
				IdleTimeoutInSeconds: 600,
			},
			ProvisionedCapacityConfig: generativeaiagentv1beta1.AgentEndpointProvisionedCapacityConfig{
				ProvisionedCapacityId: "ocid1.provisionedcapacity.oc1..example",
				PlatformRuntimeConfig: generativeaiagentv1beta1.AgentEndpointProvisionedCapacityConfigPlatformRuntimeConfig{
					PlatformRuntimeConfigType: "AGENT_PLATFORM",
					Version:                   "2024.05.31",
				},
				ToolRuntimeConfigs: []generativeaiagentv1beta1.AgentEndpointProvisionedCapacityConfigToolRuntimeConfig{
					{
						ToolRuntimeConfigType: "RAG",
						Version:               "1.1",
					},
				},
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

func makeSDKAgentEndpoint(
	t *testing.T,
	id string,
	resource *generativeaiagentv1beta1.AgentEndpoint,
	state generativeaiagentsdk.AgentEndpointLifecycleStateEnum,
) generativeaiagentsdk.AgentEndpoint {
	t.Helper()

	now := common.SDKTime{Time: time.Unix(0, 0).UTC()}
	details, err := buildAgentEndpointCreateDetails(context.Background(), resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildAgentEndpointCreateDetails() error = %v", err)
	}
	return generativeaiagentsdk.AgentEndpoint{
		Id:                        common.String(id),
		CompartmentId:             details.CompartmentId,
		AgentId:                   details.AgentId,
		TimeCreated:               &now,
		LifecycleState:            state,
		DisplayName:               details.DisplayName,
		Description:               details.Description,
		ContentModerationConfig:   details.ContentModerationConfig,
		GuardrailConfig:           details.GuardrailConfig,
		Metadata:                  maps.Clone(details.Metadata),
		HumanInputConfig:          details.HumanInputConfig,
		OutputConfig:              details.OutputConfig,
		ShouldEnableTrace:         details.ShouldEnableTrace,
		ShouldEnableCitation:      details.ShouldEnableCitation,
		ShouldEnableSession:       details.ShouldEnableSession,
		ShouldEnableMultiLanguage: details.ShouldEnableMultiLanguage,
		SessionConfig:             details.SessionConfig,
		TimeUpdated:               &now,
		LifecycleDetails:          common.String("lifecycle detail"),
		ProvisionedCapacityConfig: details.ProvisionedCapacityConfig,
		FreeformTags:              maps.Clone(details.FreeformTags),
		DefinedTags:               cloneAgentEndpointDefinedTags(details.DefinedTags),
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {
				"free-tier-retained": "true",
			},
		},
	}
}

func makeSDKAgentEndpointSummary(
	t *testing.T,
	id string,
	resource *generativeaiagentv1beta1.AgentEndpoint,
	state generativeaiagentsdk.AgentEndpointLifecycleStateEnum,
) generativeaiagentsdk.AgentEndpointSummary {
	t.Helper()

	now := common.SDKTime{Time: time.Unix(0, 0).UTC()}
	details, err := buildAgentEndpointCreateDetails(context.Background(), resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildAgentEndpointCreateDetails() error = %v", err)
	}
	return generativeaiagentsdk.AgentEndpointSummary{
		Id:                        common.String(id),
		CompartmentId:             details.CompartmentId,
		AgentId:                   details.AgentId,
		TimeCreated:               &now,
		LifecycleState:            state,
		DisplayName:               details.DisplayName,
		Description:               details.Description,
		ContentModerationConfig:   details.ContentModerationConfig,
		GuardrailConfig:           details.GuardrailConfig,
		Metadata:                  maps.Clone(details.Metadata),
		HumanInputConfig:          details.HumanInputConfig,
		OutputConfig:              details.OutputConfig,
		ShouldEnableTrace:         details.ShouldEnableTrace,
		ShouldEnableCitation:      details.ShouldEnableCitation,
		ShouldEnableSession:       details.ShouldEnableSession,
		ShouldEnableMultiLanguage: details.ShouldEnableMultiLanguage,
		SessionConfig:             details.SessionConfig,
		TimeUpdated:               &now,
		LifecycleDetails:          common.String("lifecycle detail"),
		ProvisionedCapacityConfig: details.ProvisionedCapacityConfig,
		FreeformTags:              maps.Clone(details.FreeformTags),
		DefinedTags:               cloneAgentEndpointDefinedTags(details.DefinedTags),
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {
				"free-tier-retained": "true",
			},
		},
	}
}

func makeAgentEndpointWorkRequest(
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
		CompartmentId:   common.String("ocid1.compartment.oc1..example"),
		Resources:       []generativeaiagentsdk.WorkRequestResource{{EntityType: common.String("AgentEndpoint"), ActionType: action, Identifier: common.String(resourceID)}},
		PercentComplete: &percentComplete,
		TimeAccepted:    &now,
	}
}

func cloneAgentEndpointDefinedTags(input map[string]map[string]interface{}) map[string]map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(input))
	for namespace, values := range input {
		cloned := make(map[string]interface{}, len(values))
		for key, value := range values {
			cloned[key] = value
		}
		out[namespace] = cloned
	}
	return out
}

func assertAgentEndpointStringSliceEqual(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func requireAgentEndpointStringPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %q", name, got, want)
	}
}

func requireAgentEndpointOptionalBoolPtr(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %t", name, got, want)
	}
}

func requireAgentEndpointOutputLocationPrefix(
	t *testing.T,
	name string,
	got generativeaiagentsdk.OutputLocation,
	want generativeaiagentv1beta1.AgentEndpointOutputConfigOutputLocation,
) {
	t.Helper()
	location, ok := got.(generativeaiagentsdk.ObjectStoragePrefixOutputLocation)
	if !ok {
		t.Fatalf("%s type = %T, want ObjectStoragePrefixOutputLocation", name, got)
	}
	requireAgentEndpointStringPtr(t, name+".namespaceName", location.NamespaceName, want.NamespaceName)
	requireAgentEndpointStringPtr(t, name+".bucketName", location.BucketName, want.BucketName)
	requireAgentEndpointStringPtr(t, name+".prefix", location.Prefix, want.Prefix)
}

func requireAgentEndpointAsyncCurrent(
	t *testing.T,
	resource *generativeaiagentv1beta1.AgentEndpoint,
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

func TestCloneAgentEndpointDefinedTags(t *testing.T) {
	t.Parallel()

	input := map[string]map[string]interface{}{
		"Operations": {
			"CostCenter": "42",
		},
	}
	cloned := cloneAgentEndpointDefinedTags(input)
	if !reflect.DeepEqual(cloned, input) {
		t.Fatalf("cloneAgentEndpointDefinedTags() = %#v, want %#v", cloned, input)
	}
}
