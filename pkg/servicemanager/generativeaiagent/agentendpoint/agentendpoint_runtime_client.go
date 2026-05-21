/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package agentendpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"unicode"

	"github.com/oracle/oci-go-sdk/v65/common"
	generativeaiagentsdk "github.com/oracle/oci-go-sdk/v65/generativeaiagent"
	generativeaiagentv1beta1 "github.com/oracle/oci-service-operator/api/generativeaiagent/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

var agentEndpointWorkRequestAsyncAdapter = servicemanager.WorkRequestAsyncAdapter{
	PendingStatusTokens: []string{
		string(generativeaiagentsdk.OperationStatusAccepted),
		string(generativeaiagentsdk.OperationStatusInProgress),
		string(generativeaiagentsdk.OperationStatusWaiting),
		string(generativeaiagentsdk.OperationStatusCanceling),
	},
	SucceededStatusTokens: []string{string(generativeaiagentsdk.OperationStatusSucceeded)},
	FailedStatusTokens:    []string{string(generativeaiagentsdk.OperationStatusFailed)},
	CanceledStatusTokens:  []string{string(generativeaiagentsdk.OperationStatusCanceled)},
	AttentionStatusTokens: []string{string(generativeaiagentsdk.OperationStatusNeedsAttention)},
	CreateActionTokens:    []string{string(generativeaiagentsdk.OperationTypeCreateAgentEndpoint)},
	UpdateActionTokens:    []string{string(generativeaiagentsdk.OperationTypeUpdateAgentEndpoint)},
	DeleteActionTokens:    []string{string(generativeaiagentsdk.OperationTypeDeleteAgentEndpoint)},
}

type agentEndpointOCIClient interface {
	CreateAgentEndpoint(context.Context, generativeaiagentsdk.CreateAgentEndpointRequest) (generativeaiagentsdk.CreateAgentEndpointResponse, error)
	GetAgentEndpoint(context.Context, generativeaiagentsdk.GetAgentEndpointRequest) (generativeaiagentsdk.GetAgentEndpointResponse, error)
	ListAgentEndpoints(context.Context, generativeaiagentsdk.ListAgentEndpointsRequest) (generativeaiagentsdk.ListAgentEndpointsResponse, error)
	UpdateAgentEndpoint(context.Context, generativeaiagentsdk.UpdateAgentEndpointRequest) (generativeaiagentsdk.UpdateAgentEndpointResponse, error)
	DeleteAgentEndpoint(context.Context, generativeaiagentsdk.DeleteAgentEndpointRequest) (generativeaiagentsdk.DeleteAgentEndpointResponse, error)
	GetWorkRequest(context.Context, generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error)
}

func init() {
	registerAgentEndpointRuntimeHooksMutator(func(manager *AgentEndpointServiceManager, hooks *AgentEndpointRuntimeHooks) {
		client, initErr := newAgentEndpointSDKClient(manager)
		applyAgentEndpointRuntimeHooks(hooks, client, initErr)
	})
}

func newAgentEndpointSDKClient(manager *AgentEndpointServiceManager) (agentEndpointOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("AgentEndpoint service manager is nil")
	}
	client, err := generativeaiagentsdk.NewGenerativeAiAgentClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyAgentEndpointRuntimeHooks(
	hooks *AgentEndpointRuntimeHooks,
	client agentEndpointOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedAgentEndpointRuntimeSemantics()
	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *generativeaiagentv1beta1.AgentEndpoint,
		namespace string,
	) (any, error) {
		return buildAgentEndpointCreateDetails(ctx, resource, namespace)
	}
	hooks.BuildUpdateBody = func(
		ctx context.Context,
		resource *generativeaiagentv1beta1.AgentEndpoint,
		namespace string,
		currentResponse any,
	) (any, bool, error) {
		return buildAgentEndpointUpdateBody(ctx, resource, namespace, currentResponse)
	}
	hooks.Identity.GuardExistingBeforeCreate = guardAgentEndpointExistingBeforeCreate
	hooks.ParityHooks.ValidateCreateOnlyDrift = validateAgentEndpointCreateOnlyDriftForResponse
	hooks.Create.Fields = agentEndpointCreateFields()
	hooks.Get.Fields = agentEndpointGetFields()
	hooks.List.Fields = agentEndpointListFields()
	hooks.Update.Fields = agentEndpointUpdateFields()
	hooks.Delete.Fields = agentEndpointDeleteFields()
	hooks.Async.Adapter = agentEndpointWorkRequestAsyncAdapter
	hooks.Async.GetWorkRequest = func(ctx context.Context, workRequestID string) (any, error) {
		return getAgentEndpointWorkRequest(ctx, client, initErr, workRequestID)
	}
	hooks.Async.ResolveAction = resolveAgentEndpointGeneratedWorkRequestAction
	hooks.Async.ResolvePhase = resolveAgentEndpointGeneratedWorkRequestPhase
	hooks.Async.RecoverResourceID = recoverAgentEndpointIDFromGeneratedWorkRequest
	hooks.Async.Message = agentEndpointGeneratedWorkRequestMessage
}

func newAgentEndpointServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client agentEndpointOCIClient,
) AgentEndpointServiceClient {
	return defaultAgentEndpointServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*generativeaiagentv1beta1.AgentEndpoint](
			newAgentEndpointRuntimeConfig(log, client),
		),
	}
}

func newAgentEndpointRuntimeConfig(
	log loggerutil.OSOKLogger,
	client agentEndpointOCIClient,
) generatedruntime.Config[*generativeaiagentv1beta1.AgentEndpoint] {
	hooks := newAgentEndpointRuntimeHooksWithOCIClient(client)
	applyAgentEndpointRuntimeHooks(&hooks, client, nil)
	return buildAgentEndpointGeneratedRuntimeConfig(&AgentEndpointServiceManager{Log: log}, hooks)
}

func newAgentEndpointRuntimeHooksWithOCIClient(
	client agentEndpointOCIClient,
) AgentEndpointRuntimeHooks {
	return AgentEndpointRuntimeHooks{
		Semantics: newAgentEndpointRuntimeSemantics(),
		Create: runtimeOperationHooks[generativeaiagentsdk.CreateAgentEndpointRequest, generativeaiagentsdk.CreateAgentEndpointResponse]{
			Fields: agentEndpointCreateFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.CreateAgentEndpointRequest) (generativeaiagentsdk.CreateAgentEndpointResponse, error) {
				return client.CreateAgentEndpoint(ctx, request)
			},
		},
		Get: runtimeOperationHooks[generativeaiagentsdk.GetAgentEndpointRequest, generativeaiagentsdk.GetAgentEndpointResponse]{
			Fields: agentEndpointGetFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.GetAgentEndpointRequest) (generativeaiagentsdk.GetAgentEndpointResponse, error) {
				return client.GetAgentEndpoint(ctx, request)
			},
		},
		List: runtimeOperationHooks[generativeaiagentsdk.ListAgentEndpointsRequest, generativeaiagentsdk.ListAgentEndpointsResponse]{
			Fields: agentEndpointListFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.ListAgentEndpointsRequest) (generativeaiagentsdk.ListAgentEndpointsResponse, error) {
				return client.ListAgentEndpoints(ctx, request)
			},
		},
		Update: runtimeOperationHooks[generativeaiagentsdk.UpdateAgentEndpointRequest, generativeaiagentsdk.UpdateAgentEndpointResponse]{
			Fields: agentEndpointUpdateFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.UpdateAgentEndpointRequest) (generativeaiagentsdk.UpdateAgentEndpointResponse, error) {
				return client.UpdateAgentEndpoint(ctx, request)
			},
		},
		Delete: runtimeOperationHooks[generativeaiagentsdk.DeleteAgentEndpointRequest, generativeaiagentsdk.DeleteAgentEndpointResponse]{
			Fields: agentEndpointDeleteFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.DeleteAgentEndpointRequest) (generativeaiagentsdk.DeleteAgentEndpointResponse, error) {
				return client.DeleteAgentEndpoint(ctx, request)
			},
		},
	}
}

func reviewedAgentEndpointRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newAgentEndpointRuntimeSemantics()
	semantics.Lifecycle = generatedruntime.LifecycleSemantics{
		ProvisioningStates: []string{string(generativeaiagentsdk.AgentEndpointLifecycleStateCreating)},
		UpdatingStates:     []string{string(generativeaiagentsdk.AgentEndpointLifecycleStateUpdating)},
		ActiveStates:       []string{string(generativeaiagentsdk.AgentEndpointLifecycleStateActive)},
	}
	semantics.Delete = generatedruntime.DeleteSemantics{
		Policy:         "required",
		PendingStates:  []string{string(generativeaiagentsdk.AgentEndpointLifecycleStateDeleting)},
		TerminalStates: []string{string(generativeaiagentsdk.AgentEndpointLifecycleStateDeleted)},
	}
	semantics.List = &generatedruntime.ListSemantics{
		ResponseItemsField: "Items",
		MatchFields:        []string{"compartmentId", "agentId", "displayName"},
	}
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
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
		},
		ForceNew:      []string{"agentId", "compartmentId", "shouldEnableSession"},
		ConflictsWith: map[string][]string{},
	}
	semantics.Hooks = generatedruntime.HookSet{
		Create: []generatedruntime.Hook{
			{Helper: "tfresource.CreateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "agentEndpoint", Action: "CREATED"},
		},
		Update: []generatedruntime.Hook{
			{Helper: "tfresource.UpdateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "agentEndpoint", Action: "UPDATED"},
		},
		Delete: []generatedruntime.Hook{
			{Helper: "tfresource.DeleteResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "agentEndpoint", Action: "DELETED"},
		},
	}
	semantics.CreateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetAgentEndpoint",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Create...),
	}
	semantics.UpdateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetAgentEndpoint",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Update...),
	}
	semantics.DeleteFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetAgentEndpoint/ListAgentEndpoints confirm-delete",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Delete...),
	}
	semantics.AuxiliaryOperations = nil
	return semantics
}

func agentEndpointCreateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CreateAgentEndpointDetails", RequestName: "CreateAgentEndpointDetails", Contribution: "body"},
	}
}

func agentEndpointGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "AgentEndpointId", RequestName: "agentEndpointId", Contribution: "path", PreferResourceID: true},
	}
}

func agentEndpointListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{
			FieldName:    "CompartmentId",
			RequestName:  "compartmentId",
			Contribution: "query",
			LookupPaths:  []string{"status.compartmentId", "spec.compartmentId", "compartmentId"},
		},
		{
			FieldName:    "AgentId",
			RequestName:  "agentId",
			Contribution: "query",
			LookupPaths:  []string{"status.agentId", "spec.agentId", "agentId"},
		},
		{
			FieldName:    "DisplayName",
			RequestName:  "displayName",
			Contribution: "query",
			LookupPaths:  []string{"status.displayName", "spec.displayName", "displayName"},
		},
	}
}

func agentEndpointUpdateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "AgentEndpointId", RequestName: "agentEndpointId", Contribution: "path", PreferResourceID: true},
		{FieldName: "UpdateAgentEndpointDetails", RequestName: "UpdateAgentEndpointDetails", Contribution: "body"},
	}
}

func agentEndpointDeleteFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "AgentEndpointId", RequestName: "agentEndpointId", Contribution: "path", PreferResourceID: true},
	}
}

func guardAgentEndpointExistingBeforeCreate(
	_ context.Context,
	resource *generativeaiagentv1beta1.AgentEndpoint,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("AgentEndpoint resource is nil")
	}
	if strings.TrimSpace(resource.Spec.DisplayName) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func buildAgentEndpointCreateDetails(
	ctx context.Context,
	resource *generativeaiagentv1beta1.AgentEndpoint,
	namespace string,
) (generativeaiagentsdk.CreateAgentEndpointDetails, error) {
	if resource == nil {
		return generativeaiagentsdk.CreateAgentEndpointDetails{}, fmt.Errorf("AgentEndpoint resource is nil")
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, nil, namespace)
	if err != nil {
		return generativeaiagentsdk.CreateAgentEndpointDetails{}, err
	}

	payload, err := json.Marshal(resolvedSpec)
	if err != nil {
		return generativeaiagentsdk.CreateAgentEndpointDetails{}, fmt.Errorf("marshal resolved AgentEndpoint spec: %w", err)
	}

	var details generativeaiagentsdk.CreateAgentEndpointDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return generativeaiagentsdk.CreateAgentEndpointDetails{}, fmt.Errorf("decode AgentEndpoint create request body: %w", err)
	}
	return details, nil
}

func buildAgentEndpointUpdateBody(
	ctx context.Context,
	resource *generativeaiagentv1beta1.AgentEndpoint,
	namespace string,
	currentResponse any,
) (generativeaiagentsdk.UpdateAgentEndpointDetails, bool, error) {
	if resource == nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, false, fmt.Errorf("AgentEndpoint resource is nil")
	}

	current, err := agentEndpointFromResponse(currentResponse)
	if err != nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, false, err
	}

	desired, err := buildAgentEndpointDesiredUpdateDetails(ctx, resource, namespace)
	if err != nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, false, err
	}

	details := generativeaiagentsdk.UpdateAgentEndpointDetails{}
	updateNeeded := false

	if value, ok := agentEndpointDesiredStringUpdate(resource.Spec.DisplayName, current.DisplayName); ok {
		details.DisplayName = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredStringUpdate(resource.Spec.Description, current.Description); ok {
		details.Description = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredContentModerationConfigUpdate(desired.ContentModerationConfig, current.ContentModerationConfig); ok {
		details.ContentModerationConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredJSONConfigUpdate(desired.GuardrailConfig, current.GuardrailConfig); ok {
		details.GuardrailConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredMetadataUpdate(resource.Spec.Metadata, current.Metadata); ok {
		details.Metadata = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredHumanInputConfigUpdate(desired.HumanInputConfig, current.HumanInputConfig); ok {
		details.HumanInputConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredJSONConfigUpdate(desired.OutputConfig, current.OutputConfig); ok {
		details.OutputConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredOptionalBoolUpdate(resource.Spec.ShouldEnableTrace, current.ShouldEnableTrace); ok {
		details.ShouldEnableTrace = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredOptionalBoolUpdate(resource.Spec.ShouldEnableCitation, current.ShouldEnableCitation); ok {
		details.ShouldEnableCitation = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredOptionalBoolUpdate(resource.Spec.ShouldEnableMultiLanguage, current.ShouldEnableMultiLanguage); ok {
		details.ShouldEnableMultiLanguage = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredJSONConfigUpdate(desired.SessionConfig, current.SessionConfig); ok {
		details.SessionConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredJSONConfigUpdate(desired.ProvisionedCapacityConfig, current.ProvisionedCapacityConfig); ok {
		details.ProvisionedCapacityConfig = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredFreeformTagsUpdate(resource.Spec.FreeformTags, current.FreeformTags); ok {
		details.FreeformTags = value
		updateNeeded = true
	}
	if value, ok := agentEndpointDesiredDefinedTagsUpdate(resource.Spec.DefinedTags, current.DefinedTags); ok {
		details.DefinedTags = value
		updateNeeded = true
	}

	return details, updateNeeded, nil
}

func buildAgentEndpointDesiredUpdateDetails(
	ctx context.Context,
	resource *generativeaiagentv1beta1.AgentEndpoint,
	namespace string,
) (generativeaiagentsdk.UpdateAgentEndpointDetails, error) {
	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, nil, namespace)
	if err != nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, err
	}

	payload, err := json.Marshal(resolvedSpec)
	if err != nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, fmt.Errorf("marshal resolved AgentEndpoint spec: %w", err)
	}

	var details generativeaiagentsdk.UpdateAgentEndpointDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return generativeaiagentsdk.UpdateAgentEndpointDetails{}, fmt.Errorf("decode AgentEndpoint update request body: %w", err)
	}
	return details, nil
}

func validateAgentEndpointCreateOnlyDriftForResponse(
	resource *generativeaiagentv1beta1.AgentEndpoint,
	currentResponse any,
) error {
	if resource == nil {
		return fmt.Errorf("AgentEndpoint resource is nil")
	}
	current, err := agentEndpointFromResponse(currentResponse)
	if err != nil {
		return err
	}

	var drift []string
	if !agentEndpointStringPtrEqual(current.CompartmentId, resource.Spec.CompartmentId) {
		drift = append(drift, "compartmentId")
	}
	if !agentEndpointStringPtrEqual(current.AgentId, resource.Spec.AgentId) {
		drift = append(drift, "agentId")
	}
	if !agentEndpointOptionalBoolEqual(current.ShouldEnableSession, resource.Spec.ShouldEnableSession) {
		drift = append(drift, "shouldEnableSession")
	}
	if len(drift) == 0 {
		return nil
	}
	return fmt.Errorf("AgentEndpoint create-only field drift is not supported: %s", strings.Join(drift, ", "))
}

func agentEndpointFromResponse(currentResponse any) (generativeaiagentsdk.AgentEndpoint, error) {
	switch current := currentResponse.(type) {
	case generativeaiagentsdk.AgentEndpoint:
		return current, nil
	case *generativeaiagentsdk.AgentEndpoint:
		if current == nil {
			return generativeaiagentsdk.AgentEndpoint{}, fmt.Errorf("current AgentEndpoint response is nil")
		}
		return *current, nil
	case generativeaiagentsdk.AgentEndpointSummary:
		return generativeaiagentsdk.AgentEndpoint{
			Id:                        current.Id,
			CompartmentId:             current.CompartmentId,
			AgentId:                   current.AgentId,
			TimeCreated:               current.TimeCreated,
			LifecycleState:            current.LifecycleState,
			DisplayName:               current.DisplayName,
			Description:               current.Description,
			ContentModerationConfig:   current.ContentModerationConfig,
			GuardrailConfig:           current.GuardrailConfig,
			Metadata:                  current.Metadata,
			HumanInputConfig:          current.HumanInputConfig,
			OutputConfig:              current.OutputConfig,
			ShouldEnableTrace:         current.ShouldEnableTrace,
			ShouldEnableCitation:      current.ShouldEnableCitation,
			ShouldEnableSession:       current.ShouldEnableSession,
			ShouldEnableMultiLanguage: current.ShouldEnableMultiLanguage,
			SessionConfig:             current.SessionConfig,
			TimeUpdated:               current.TimeUpdated,
			LifecycleDetails:          current.LifecycleDetails,
			ProvisionedCapacityConfig: current.ProvisionedCapacityConfig,
			FreeformTags:              current.FreeformTags,
			DefinedTags:               current.DefinedTags,
			SystemTags:                current.SystemTags,
		}, nil
	case *generativeaiagentsdk.AgentEndpointSummary:
		if current == nil {
			return generativeaiagentsdk.AgentEndpoint{}, fmt.Errorf("current AgentEndpoint response is nil")
		}
		return agentEndpointFromResponse(*current)
	case generativeaiagentsdk.GetAgentEndpointResponse:
		return current.AgentEndpoint, nil
	case *generativeaiagentsdk.GetAgentEndpointResponse:
		if current == nil {
			return generativeaiagentsdk.AgentEndpoint{}, fmt.Errorf("current AgentEndpoint response is nil")
		}
		return current.AgentEndpoint, nil
	default:
		return generativeaiagentsdk.AgentEndpoint{}, fmt.Errorf("unexpected current AgentEndpoint response type %T", currentResponse)
	}
}

func agentEndpointDesiredStringUpdate(spec string, current *string) (*string, bool) {
	currentValue := ""
	if current != nil {
		currentValue = *current
	}
	if spec == currentValue {
		return nil, false
	}
	if spec == "" && current == nil {
		return nil, false
	}
	return common.String(spec), true
}

func agentEndpointDesiredOptionalBoolUpdate(spec bool, current *bool) (*bool, bool) {
	if current == nil && !spec {
		return nil, false
	}
	if current != nil && *current == spec {
		return nil, false
	}
	return common.Bool(spec), true
}

func agentEndpointDesiredContentModerationConfigUpdate(
	desired *generativeaiagentsdk.ContentModerationConfig,
	current *generativeaiagentsdk.ContentModerationConfig,
) (*generativeaiagentsdk.ContentModerationConfig, bool) {
	if desired == nil {
		return nil, false
	}
	if agentEndpointContentModerationConfigEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func agentEndpointContentModerationConfigEqual(
	desired *generativeaiagentsdk.ContentModerationConfig,
	current *generativeaiagentsdk.ContentModerationConfig,
) bool {
	return agentEndpointConfigBoolValue(desired, func(config *generativeaiagentsdk.ContentModerationConfig) *bool {
		return config.ShouldEnableOnInput
	}) == agentEndpointConfigBoolValue(current, func(config *generativeaiagentsdk.ContentModerationConfig) *bool {
		return config.ShouldEnableOnInput
	}) &&
		agentEndpointConfigBoolValue(desired, func(config *generativeaiagentsdk.ContentModerationConfig) *bool {
			return config.ShouldEnableOnOutput
		}) == agentEndpointConfigBoolValue(current, func(config *generativeaiagentsdk.ContentModerationConfig) *bool {
			return config.ShouldEnableOnOutput
		})
}

func agentEndpointDesiredHumanInputConfigUpdate(
	desired *generativeaiagentsdk.HumanInputConfig,
	current *generativeaiagentsdk.HumanInputConfig,
) (*generativeaiagentsdk.HumanInputConfig, bool) {
	if desired == nil {
		return nil, false
	}
	if agentEndpointHumanInputConfigEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func agentEndpointHumanInputConfigEqual(
	desired *generativeaiagentsdk.HumanInputConfig,
	current *generativeaiagentsdk.HumanInputConfig,
) bool {
	return agentEndpointConfigBoolValue(desired, func(config *generativeaiagentsdk.HumanInputConfig) *bool {
		return config.ShouldEnableHumanInput
	}) == agentEndpointConfigBoolValue(current, func(config *generativeaiagentsdk.HumanInputConfig) *bool {
		return config.ShouldEnableHumanInput
	})
}

func agentEndpointDesiredJSONConfigUpdate[T any](desired *T, current *T) (*T, bool) {
	if desired == nil {
		return nil, false
	}
	if agentEndpointJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func agentEndpointDesiredMetadataUpdate(
	spec map[string]string,
	current map[string]string,
) (map[string]string, bool) {
	if spec == nil {
		return nil, false
	}
	if len(spec) == 0 && len(current) == 0 {
		return nil, false
	}
	if maps.Equal(spec, current) {
		return nil, false
	}
	return maps.Clone(spec), true
}

func agentEndpointDesiredFreeformTagsUpdate(
	spec map[string]string,
	current map[string]string,
) (map[string]string, bool) {
	if spec == nil {
		return nil, false
	}
	if len(spec) == 0 && len(current) == 0 {
		return nil, false
	}
	if maps.Equal(spec, current) {
		return nil, false
	}
	return maps.Clone(spec), true
}

func agentEndpointDesiredDefinedTagsUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) (map[string]map[string]interface{}, bool) {
	if spec == nil {
		return nil, false
	}

	desired := agentEndpointDefinedTagsFromSpec(spec)
	if len(desired) == 0 && len(current) == 0 {
		return nil, false
	}
	if agentEndpointJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func agentEndpointDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
	if spec == nil {
		return nil
	}

	desired := make(map[string]map[string]interface{}, len(spec))
	for namespace, values := range spec {
		converted := make(map[string]interface{}, len(values))
		for key, value := range values {
			converted[key] = value
		}
		desired[namespace] = converted
	}
	return desired
}

func agentEndpointJSONEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func getAgentEndpointWorkRequest(
	ctx context.Context,
	client agentEndpointOCIClient,
	initErr error,
	workRequestID string,
) (any, error) {
	if initErr != nil {
		return nil, fmt.Errorf("initialize AgentEndpoint OCI client: %w", initErr)
	}
	if client == nil {
		return nil, fmt.Errorf("AgentEndpoint OCI client is not configured")
	}

	response, err := client.GetWorkRequest(ctx, generativeaiagentsdk.GetWorkRequestRequest{
		WorkRequestId: common.String(strings.TrimSpace(workRequestID)),
	})
	if err != nil {
		return nil, err
	}
	return response.WorkRequest, nil
}

func resolveAgentEndpointGeneratedWorkRequestAction(workRequest any) (string, error) {
	current, err := agentEndpointWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}
	return string(current.OperationType), nil
}

func resolveAgentEndpointGeneratedWorkRequestPhase(workRequest any) (shared.OSOKAsyncPhase, bool, error) {
	current, err := agentEndpointWorkRequestFromAny(workRequest)
	if err != nil {
		return "", false, err
	}
	phase, ok := agentEndpointWorkRequestPhaseFromOperationType(current.OperationType)
	return phase, ok, nil
}

func recoverAgentEndpointIDFromGeneratedWorkRequest(
	_ *generativeaiagentv1beta1.AgentEndpoint,
	workRequest any,
	phase shared.OSOKAsyncPhase,
) (string, error) {
	current, err := agentEndpointWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}

	action := agentEndpointWorkRequestActionForPhase(phase)
	if id, ok := resolveAgentEndpointIDFromResources(current.Resources, action, true); ok {
		return id, nil
	}
	if id, ok := resolveAgentEndpointIDFromResources(current.Resources, action, false); ok {
		return id, nil
	}
	return "", fmt.Errorf("AgentEndpoint work request %s does not expose an agent endpoint identifier", agentEndpointStringValue(current.Id))
}

func agentEndpointGeneratedWorkRequestMessage(phase shared.OSOKAsyncPhase, workRequest any) string {
	current, err := agentEndpointWorkRequestFromAny(workRequest)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("AgentEndpoint %s work request %s is %s", phase, agentEndpointStringValue(current.Id), current.Status)
}

func agentEndpointWorkRequestFromAny(workRequest any) (generativeaiagentsdk.WorkRequest, error) {
	switch current := workRequest.(type) {
	case generativeaiagentsdk.WorkRequest:
		return current, nil
	case *generativeaiagentsdk.WorkRequest:
		if current == nil {
			return generativeaiagentsdk.WorkRequest{}, fmt.Errorf("AgentEndpoint work request is nil")
		}
		return *current, nil
	default:
		return generativeaiagentsdk.WorkRequest{}, fmt.Errorf("unexpected AgentEndpoint work request type %T", workRequest)
	}
}

func agentEndpointWorkRequestPhaseFromOperationType(
	operationType generativeaiagentsdk.OperationTypeEnum,
) (shared.OSOKAsyncPhase, bool) {
	switch operationType {
	case generativeaiagentsdk.OperationTypeCreateAgentEndpoint:
		return shared.OSOKAsyncPhaseCreate, true
	case generativeaiagentsdk.OperationTypeUpdateAgentEndpoint:
		return shared.OSOKAsyncPhaseUpdate, true
	case generativeaiagentsdk.OperationTypeDeleteAgentEndpoint:
		return shared.OSOKAsyncPhaseDelete, true
	default:
		return "", false
	}
}

func agentEndpointWorkRequestActionForPhase(
	phase shared.OSOKAsyncPhase,
) generativeaiagentsdk.ActionTypeEnum {
	switch phase {
	case shared.OSOKAsyncPhaseCreate:
		return generativeaiagentsdk.ActionTypeCreated
	case shared.OSOKAsyncPhaseUpdate:
		return generativeaiagentsdk.ActionTypeUpdated
	case shared.OSOKAsyncPhaseDelete:
		return generativeaiagentsdk.ActionTypeDeleted
	default:
		return ""
	}
}

func resolveAgentEndpointIDFromResources(
	resources []generativeaiagentsdk.WorkRequestResource,
	action generativeaiagentsdk.ActionTypeEnum,
	preferAgentEndpointOnly bool,
) (string, bool) {
	var candidate string
	for _, resource := range resources {
		if action != "" && resource.ActionType != action {
			continue
		}
		if preferAgentEndpointOnly && !isAgentEndpointWorkRequestResource(resource) {
			continue
		}
		id := strings.TrimSpace(agentEndpointStringValue(resource.Identifier))
		if id == "" {
			continue
		}
		if candidate == "" {
			candidate = id
			continue
		}
		if candidate != id {
			return "", false
		}
	}
	return candidate, candidate != ""
}

func isAgentEndpointWorkRequestResource(resource generativeaiagentsdk.WorkRequestResource) bool {
	return normalizeAgentEndpointWorkRequestToken(agentEndpointStringValue(resource.EntityType)) == "agentendpoint"
}

func normalizeAgentEndpointWorkRequestToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func agentEndpointStringPtrEqual(current *string, desired string) bool {
	return strings.TrimSpace(agentEndpointStringValue(current)) == strings.TrimSpace(desired)
}

func agentEndpointOptionalBoolEqual(current *bool, desired bool) bool {
	if current == nil {
		return !desired
	}
	return *current == desired
}

func agentEndpointConfigBoolValue[T any](config *T, selector func(*T) *bool) bool {
	if config == nil {
		return false
	}
	return agentEndpointOptionalBoolValue(selector(config))
}

func agentEndpointOptionalBoolValue(value *bool) bool {
	return value != nil && *value
}

func agentEndpointStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
