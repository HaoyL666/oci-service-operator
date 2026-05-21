/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package mediaworkflowconfiguration

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	mediaservicessdk "github.com/oracle/oci-go-sdk/v65/mediaservices"
	mediaservicesv1beta1 "github.com/oracle/oci-service-operator/api/mediaservices/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const mediaWorkflowConfigurationDeletePendingMessage = "OCI resource delete is in progress"

type mediaWorkflowConfigurationOCIClient interface {
	CreateMediaWorkflowConfiguration(context.Context, mediaservicessdk.CreateMediaWorkflowConfigurationRequest) (mediaservicessdk.CreateMediaWorkflowConfigurationResponse, error)
	GetMediaWorkflowConfiguration(context.Context, mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error)
	ListMediaWorkflowConfigurations(context.Context, mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error)
	UpdateMediaWorkflowConfiguration(context.Context, mediaservicessdk.UpdateMediaWorkflowConfigurationRequest) (mediaservicessdk.UpdateMediaWorkflowConfigurationResponse, error)
	DeleteMediaWorkflowConfiguration(context.Context, mediaservicessdk.DeleteMediaWorkflowConfigurationRequest) (mediaservicessdk.DeleteMediaWorkflowConfigurationResponse, error)
}

func init() {
	registerMediaWorkflowConfigurationRuntimeHooksMutator(func(_ *MediaWorkflowConfigurationServiceManager, hooks *MediaWorkflowConfigurationRuntimeHooks) {
		applyMediaWorkflowConfigurationRuntimeHooks(hooks)
	})
}

func applyMediaWorkflowConfigurationRuntimeHooks(hooks *MediaWorkflowConfigurationRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedMediaWorkflowConfigurationRuntimeSemantics()
	hooks.Identity.GuardExistingBeforeCreate = guardMediaWorkflowConfigurationExistingBeforeCreate
	hooks.ParityHooks.NormalizeDesiredState = normalizeMediaWorkflowConfigurationDesiredState
	hooks.ParityHooks.ValidateCreateOnlyDrift = validateMediaWorkflowConfigurationCreateOnlyDrift
	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
		namespace string,
	) (any, error) {
		return buildMediaWorkflowConfigurationCreateDetails(ctx, resource, namespace)
	}
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildMediaWorkflowConfigurationUpdateBody(resource, currentResponse)
	}
	hooks.Create.Fields = mediaWorkflowConfigurationCreateFields()
	hooks.Get.Fields = mediaWorkflowConfigurationGetFields()
	hooks.List.Fields = mediaWorkflowConfigurationListFields()
	wrapMediaWorkflowConfigurationListPages(hooks)
	hooks.Update.Fields = mediaWorkflowConfigurationUpdateFields()
	hooks.Delete.Fields = mediaWorkflowConfigurationDeleteFields()
	hooks.DeleteHooks.ApplyOutcome = applyMediaWorkflowConfigurationDeleteOutcome
}

func newMediaWorkflowConfigurationServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client mediaWorkflowConfigurationOCIClient,
) MediaWorkflowConfigurationServiceClient {
	hooks := newMediaWorkflowConfigurationRuntimeHooksWithOCIClient(client)
	applyMediaWorkflowConfigurationRuntimeHooks(&hooks)
	delegate := defaultMediaWorkflowConfigurationServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*mediaservicesv1beta1.MediaWorkflowConfiguration](
			buildMediaWorkflowConfigurationGeneratedRuntimeConfig(&MediaWorkflowConfigurationServiceManager{Log: log}, hooks),
		),
	}
	return wrapMediaWorkflowConfigurationGeneratedClient(hooks, delegate)
}

func newMediaWorkflowConfigurationRuntimeHooksWithOCIClient(client mediaWorkflowConfigurationOCIClient) MediaWorkflowConfigurationRuntimeHooks {
	return MediaWorkflowConfigurationRuntimeHooks{
		Create: runtimeOperationHooks[mediaservicessdk.CreateMediaWorkflowConfigurationRequest, mediaservicessdk.CreateMediaWorkflowConfigurationResponse]{
			Fields: mediaWorkflowConfigurationCreateFields(),
			Call: func(ctx context.Context, request mediaservicessdk.CreateMediaWorkflowConfigurationRequest) (mediaservicessdk.CreateMediaWorkflowConfigurationResponse, error) {
				return client.CreateMediaWorkflowConfiguration(ctx, request)
			},
		},
		Get: runtimeOperationHooks[mediaservicessdk.GetMediaWorkflowConfigurationRequest, mediaservicessdk.GetMediaWorkflowConfigurationResponse]{
			Fields: mediaWorkflowConfigurationGetFields(),
			Call: func(ctx context.Context, request mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error) {
				return client.GetMediaWorkflowConfiguration(ctx, request)
			},
		},
		List: runtimeOperationHooks[mediaservicessdk.ListMediaWorkflowConfigurationsRequest, mediaservicessdk.ListMediaWorkflowConfigurationsResponse]{
			Fields: mediaWorkflowConfigurationListFields(),
			Call: func(ctx context.Context, request mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error) {
				return client.ListMediaWorkflowConfigurations(ctx, request)
			},
		},
		Update: runtimeOperationHooks[mediaservicessdk.UpdateMediaWorkflowConfigurationRequest, mediaservicessdk.UpdateMediaWorkflowConfigurationResponse]{
			Fields: mediaWorkflowConfigurationUpdateFields(),
			Call: func(ctx context.Context, request mediaservicessdk.UpdateMediaWorkflowConfigurationRequest) (mediaservicessdk.UpdateMediaWorkflowConfigurationResponse, error) {
				return client.UpdateMediaWorkflowConfiguration(ctx, request)
			},
		},
		Delete: runtimeOperationHooks[mediaservicessdk.DeleteMediaWorkflowConfigurationRequest, mediaservicessdk.DeleteMediaWorkflowConfigurationResponse]{
			Fields: mediaWorkflowConfigurationDeleteFields(),
			Call: func(ctx context.Context, request mediaservicessdk.DeleteMediaWorkflowConfigurationRequest) (mediaservicessdk.DeleteMediaWorkflowConfigurationResponse, error) {
				return client.DeleteMediaWorkflowConfiguration(ctx, request)
			},
		},
	}
}

func reviewedMediaWorkflowConfigurationRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newMediaWorkflowConfigurationRuntimeSemantics()
	semantics.Lifecycle = generatedruntime.LifecycleSemantics{
		ActiveStates: []string{string(mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive)},
	}
	semantics.Delete = generatedruntime.DeleteSemantics{
		Policy:         "required",
		TerminalStates: []string{string(mediaservicessdk.MediaWorkflowConfigurationLifecycleStateDeleted)},
	}
	semantics.List = &generatedruntime.ListSemantics{
		ResponseItemsField: "Items",
		MatchFields:        []string{"compartmentId", "displayName"},
	}
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
			"definedTags",
			"displayName",
			"freeformTags",
			"parameters",
		},
		ForceNew: []string{
			"compartmentId",
		},
		ConflictsWith: map[string][]string{},
	}
	semantics.Hooks = generatedruntime.HookSet{
		Create: []generatedruntime.Hook{{Helper: "tfresource.CreateResource", EntityType: "MediaWorkflowConfiguration", Action: "CreateMediaWorkflowConfiguration"}},
		Update: []generatedruntime.Hook{{Helper: "tfresource.UpdateResource", EntityType: "MediaWorkflowConfiguration", Action: "UpdateMediaWorkflowConfiguration"}},
		Delete: []generatedruntime.Hook{{Helper: "tfresource.DeleteResource", EntityType: "MediaWorkflowConfiguration", Action: "DeleteMediaWorkflowConfiguration"}},
	}
	semantics.CreateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "read-after-write",
		Hooks:    []generatedruntime.Hook{{Helper: "tfresource.CreateResource", EntityType: "MediaWorkflowConfiguration", Action: "GetMediaWorkflowConfiguration"}},
	}
	semantics.UpdateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "read-after-write",
		Hooks:    []generatedruntime.Hook{{Helper: "tfresource.UpdateResource", EntityType: "MediaWorkflowConfiguration", Action: "GetMediaWorkflowConfiguration"}},
	}
	semantics.DeleteFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "confirm-delete",
		Hooks:    []generatedruntime.Hook{{Helper: "tfresource.DeleteResource", EntityType: "MediaWorkflowConfiguration", Action: "GetMediaWorkflowConfiguration"}},
	}
	semantics.AuxiliaryOperations = nil
	semantics.Unsupported = nil
	return semantics
}

func mediaWorkflowConfigurationCreateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CreateMediaWorkflowConfigurationDetails", RequestName: "CreateMediaWorkflowConfigurationDetails", Contribution: "body"},
	}
}

func mediaWorkflowConfigurationGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MediaWorkflowConfigurationId", RequestName: "mediaWorkflowConfigurationId", Contribution: "path", PreferResourceID: true},
	}
}

func mediaWorkflowConfigurationListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{
			FieldName:    "CompartmentId",
			RequestName:  "compartmentId",
			Contribution: "query",
			LookupPaths:  []string{"status.compartmentId", "spec.compartmentId", "compartmentId"},
		},
		{
			FieldName:    "DisplayName",
			RequestName:  "displayName",
			Contribution: "query",
			LookupPaths:  []string{"status.displayName", "spec.displayName", "displayName"},
		},
	}
}

func mediaWorkflowConfigurationUpdateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MediaWorkflowConfigurationId", RequestName: "mediaWorkflowConfigurationId", Contribution: "path", PreferResourceID: true},
		{FieldName: "UpdateMediaWorkflowConfigurationDetails", RequestName: "UpdateMediaWorkflowConfigurationDetails", Contribution: "body"},
	}
}

func mediaWorkflowConfigurationDeleteFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MediaWorkflowConfigurationId", RequestName: "mediaWorkflowConfigurationId", Contribution: "path", PreferResourceID: true},
	}
}

func wrapMediaWorkflowConfigurationListPages(hooks *MediaWorkflowConfigurationRuntimeHooks) {
	if hooks == nil || hooks.List.Call == nil {
		return
	}

	call := hooks.List.Call
	hooks.List.Call = func(ctx context.Context, request mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error) {
		return listMediaWorkflowConfigurationPages(ctx, call, request)
	}
}

func listMediaWorkflowConfigurationPages(
	ctx context.Context,
	call func(context.Context, mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error),
	request mediaservicessdk.ListMediaWorkflowConfigurationsRequest,
) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error) {
	var combined mediaservicessdk.ListMediaWorkflowConfigurationsResponse
	for {
		response, err := call(ctx, request)
		if err != nil {
			return response, err
		}
		if combined.OpcRequestId == nil {
			combined.OpcRequestId = response.OpcRequestId
		}
		combined.RawResponse = response.RawResponse
		combined.Items = append(combined.Items, response.Items...)
		if response.OpcNextPage == nil || strings.TrimSpace(*response.OpcNextPage) == "" {
			combined.OpcNextPage = nil
			return combined, nil
		}
		request.Page = response.OpcNextPage
	}
}

func guardMediaWorkflowConfigurationExistingBeforeCreate(
	_ context.Context,
	resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("MediaWorkflowConfiguration resource is nil")
	}
	if strings.TrimSpace(resource.Spec.CompartmentId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("MediaWorkflowConfiguration spec.compartmentId is required")
	}
	if strings.TrimSpace(resource.Spec.DisplayName) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func buildMediaWorkflowConfigurationCreateDetails(
	ctx context.Context,
	resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
	namespace string,
) (mediaservicessdk.CreateMediaWorkflowConfigurationDetails, error) {
	if resource == nil {
		return mediaservicessdk.CreateMediaWorkflowConfigurationDetails{}, fmt.Errorf("MediaWorkflowConfiguration resource is nil")
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, nil, namespace)
	if err != nil {
		return mediaservicessdk.CreateMediaWorkflowConfigurationDetails{}, err
	}

	payload, err := json.Marshal(resolvedSpec)
	if err != nil {
		return mediaservicessdk.CreateMediaWorkflowConfigurationDetails{}, fmt.Errorf("marshal resolved MediaWorkflowConfiguration spec: %w", err)
	}

	var details mediaservicessdk.CreateMediaWorkflowConfigurationDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return mediaservicessdk.CreateMediaWorkflowConfigurationDetails{}, fmt.Errorf("decode MediaWorkflowConfiguration create request body: %w", err)
	}
	for index := range details.Locks {
		details.Locks[index].TimeCreated = nil
	}
	return details, nil
}

func buildMediaWorkflowConfigurationUpdateBody(
	resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
	currentResponse any,
) (mediaservicessdk.UpdateMediaWorkflowConfigurationDetails, bool, error) {
	if resource == nil {
		return mediaservicessdk.UpdateMediaWorkflowConfigurationDetails{}, false, fmt.Errorf("MediaWorkflowConfiguration resource is nil")
	}

	current, err := mediaWorkflowConfigurationRuntimeBody(currentResponse)
	if err != nil {
		return mediaservicessdk.UpdateMediaWorkflowConfigurationDetails{}, false, err
	}

	spec := resource.Spec
	details := mediaservicessdk.UpdateMediaWorkflowConfigurationDetails{}
	updateNeeded := false

	if desired, ok := mediaWorkflowConfigurationDesiredStringUpdate(spec.DisplayName, current.DisplayName); ok {
		details.DisplayName = desired
		updateNeeded = true
	}
	if desired, ok, err := mediaWorkflowConfigurationDesiredParametersUpdate(spec.Parameters, current.Parameters); err != nil {
		return mediaservicessdk.UpdateMediaWorkflowConfigurationDetails{}, false, err
	} else if ok {
		details.Parameters = desired
		updateNeeded = true
	}
	if desired, ok := mediaWorkflowConfigurationDesiredFreeformTagsUpdate(spec.FreeformTags, current.FreeformTags); ok {
		details.FreeformTags = desired
		updateNeeded = true
	}
	if desired, ok, err := mediaWorkflowConfigurationDesiredDefinedTagsUpdate(spec.DefinedTags, current.DefinedTags); err != nil {
		return mediaservicessdk.UpdateMediaWorkflowConfigurationDetails{}, false, err
	} else if ok {
		details.DefinedTags = desired
		updateNeeded = true
	}

	return details, updateNeeded, nil
}

func mediaWorkflowConfigurationRuntimeBody(currentResponse any) (mediaservicessdk.MediaWorkflowConfiguration, error) {
	switch current := currentResponse.(type) {
	case mediaservicessdk.MediaWorkflowConfiguration:
		return current, nil
	case *mediaservicessdk.MediaWorkflowConfiguration:
		if current == nil {
			return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("current MediaWorkflowConfiguration response is nil")
		}
		return *current, nil
	case mediaservicessdk.MediaWorkflowConfigurationSummary:
		return mediaservicessdk.MediaWorkflowConfiguration{
			Id:              current.Id,
			DisplayName:     current.DisplayName,
			CompartmentId:   current.CompartmentId,
			Parameters:      nil,
			TimeCreated:     current.TimeCreated,
			TimeUpdated:     current.TimeUpdated,
			LifecycleState:  current.LifecycleState,
			LifecyleDetails: current.LifecycleDetails,
			FreeformTags:    current.FreeformTags,
			DefinedTags:     current.DefinedTags,
			SystemTags:      current.SystemTags,
			Locks:           current.Locks,
		}, nil
	case *mediaservicessdk.MediaWorkflowConfigurationSummary:
		if current == nil {
			return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("current MediaWorkflowConfiguration response is nil")
		}
		return mediaWorkflowConfigurationRuntimeBody(*current)
	case mediaservicessdk.CreateMediaWorkflowConfigurationResponse:
		return current.MediaWorkflowConfiguration, nil
	case *mediaservicessdk.CreateMediaWorkflowConfigurationResponse:
		if current == nil {
			return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("current MediaWorkflowConfiguration response is nil")
		}
		return current.MediaWorkflowConfiguration, nil
	case mediaservicessdk.GetMediaWorkflowConfigurationResponse:
		return current.MediaWorkflowConfiguration, nil
	case *mediaservicessdk.GetMediaWorkflowConfigurationResponse:
		if current == nil {
			return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("current MediaWorkflowConfiguration response is nil")
		}
		return current.MediaWorkflowConfiguration, nil
	case mediaservicessdk.UpdateMediaWorkflowConfigurationResponse:
		return current.MediaWorkflowConfiguration, nil
	case *mediaservicessdk.UpdateMediaWorkflowConfigurationResponse:
		if current == nil {
			return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("current MediaWorkflowConfiguration response is nil")
		}
		return current.MediaWorkflowConfiguration, nil
	default:
		return mediaservicessdk.MediaWorkflowConfiguration{}, fmt.Errorf("unexpected current MediaWorkflowConfiguration response type %T", currentResponse)
	}
}

func applyMediaWorkflowConfigurationDeleteOutcome(
	resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
	response any,
	stage generatedruntime.DeleteConfirmStage,
) (generatedruntime.DeleteOutcome, error) {
	lifecycleState := strings.ToUpper(mediaWorkflowConfigurationLifecycleState(response))
	if lifecycleState != string(mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive) {
		return generatedruntime.DeleteOutcome{}, nil
	}

	if stage == generatedruntime.DeleteConfirmStageAlreadyPending &&
		!mediaWorkflowConfigurationDeleteAlreadyPending(resource) {
		return generatedruntime.DeleteOutcome{}, nil
	}

	if stage == generatedruntime.DeleteConfirmStageAfterRequest ||
		stage == generatedruntime.DeleteConfirmStageAlreadyPending {
		markMediaWorkflowConfigurationTerminating(resource, response)
		return generatedruntime.DeleteOutcome{Handled: true, Deleted: false}, nil
	}
	return generatedruntime.DeleteOutcome{}, nil
}

func mediaWorkflowConfigurationDeleteAlreadyPending(resource *mediaservicesv1beta1.MediaWorkflowConfiguration) bool {
	if resource == nil {
		return false
	}
	current := resource.Status.OsokStatus.Async.Current
	return current != nil &&
		current.Phase == shared.OSOKAsyncPhaseDelete &&
		current.NormalizedClass == shared.OSOKAsyncClassPending
}

func markMediaWorkflowConfigurationTerminating(
	resource *mediaservicesv1beta1.MediaWorkflowConfiguration,
	response any,
) {
	if resource == nil {
		return
	}

	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Message = mediaWorkflowConfigurationDeletePendingMessage
	status.Reason = string(shared.Terminating)
	status.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseDelete,
		RawStatus:       mediaWorkflowConfigurationLifecycleState(response),
		NormalizedClass: shared.OSOKAsyncClassPending,
		Message:         mediaWorkflowConfigurationDeletePendingMessage,
		UpdatedAt:       &now,
	}
	*status = util.UpdateOSOKStatusCondition(
		*status,
		shared.Terminating,
		corev1.ConditionTrue,
		"",
		mediaWorkflowConfigurationDeletePendingMessage,
		loggerutil.OSOKLogger{},
	)
}

func mediaWorkflowConfigurationLifecycleState(response any) string {
	current, err := mediaWorkflowConfigurationRuntimeBody(response)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(current.LifecycleState))
}

func normalizeMediaWorkflowConfigurationDesiredState(resource *mediaservicesv1beta1.MediaWorkflowConfiguration, currentResponse any) {
	if resource == nil || resource.Spec.Locks == nil {
		return
	}
	current, err := mediaWorkflowConfigurationRuntimeBody(currentResponse)
	if err != nil {
		return
	}
	normalized, ok := mediaWorkflowConfigurationCanonicalizeDesiredLocks(resource.Spec.Locks, current.Locks)
	if !ok {
		return
	}
	resource.Spec.Locks = normalized
}

func validateMediaWorkflowConfigurationCreateOnlyDrift(resource *mediaservicesv1beta1.MediaWorkflowConfiguration, currentResponse any) error {
	if resource == nil {
		return nil
	}
	current, err := mediaWorkflowConfigurationRuntimeBody(currentResponse)
	if err != nil {
		return err
	}
	if resource.Spec.Locks == nil {
		if len(current.Locks) == 0 {
			return nil
		}
		return fmt.Errorf("MediaWorkflowConfiguration create-only drift detected for locks; removing or omitting spec.locks after create is not supported while OCI still reports locks")
	}
	if mediaWorkflowConfigurationLocksEqual(resource.Spec.Locks, current.Locks) {
		return nil
	}
	return fmt.Errorf("MediaWorkflowConfiguration create-only drift detected for locks; replace the resource or restore the desired spec before update")
}

func mediaWorkflowConfigurationDesiredStringUpdate(spec string, current *string) (*string, bool) {
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

func mediaWorkflowConfigurationDesiredParametersUpdate(
	spec map[string]shared.JSONValue,
	current map[string]interface{},
) (map[string]interface{}, bool, error) {
	if spec == nil {
		return nil, false, nil
	}
	if len(spec) == 0 && len(current) == 0 {
		return nil, false, nil
	}

	desired, err := mediaWorkflowConfigurationParametersFromSpec(spec)
	if err != nil {
		return nil, false, err
	}
	desiredJSON, err := mediaWorkflowConfigurationCanonicalJSONString(desired)
	if err != nil {
		return nil, false, fmt.Errorf("normalize desired MediaWorkflowConfiguration parameters: %w", err)
	}
	currentJSON, err := mediaWorkflowConfigurationCanonicalJSONString(current)
	if err != nil {
		return nil, false, fmt.Errorf("normalize current MediaWorkflowConfiguration parameters: %w", err)
	}
	if desiredJSON == currentJSON {
		return nil, false, nil
	}
	return desired, true, nil
}

func mediaWorkflowConfigurationDesiredFreeformTagsUpdate(
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

func mediaWorkflowConfigurationDesiredDefinedTagsUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) (map[string]map[string]interface{}, bool, error) {
	if spec == nil {
		return nil, false, nil
	}
	if len(spec) == 0 && len(current) == 0 {
		return nil, false, nil
	}

	desired, err := mediaWorkflowConfigurationDefinedTagsFromSpec(spec)
	if err != nil {
		return nil, false, err
	}
	desiredJSON, err := mediaWorkflowConfigurationCanonicalJSONString(desired)
	if err != nil {
		return nil, false, fmt.Errorf("normalize desired MediaWorkflowConfiguration definedTags: %w", err)
	}
	currentJSON, err := mediaWorkflowConfigurationCanonicalJSONString(current)
	if err != nil {
		return nil, false, fmt.Errorf("normalize current MediaWorkflowConfiguration definedTags: %w", err)
	}
	if desiredJSON == currentJSON {
		return nil, false, nil
	}
	return desired, true, nil
}

func mediaWorkflowConfigurationParametersFromSpec(spec map[string]shared.JSONValue) (map[string]interface{}, error) {
	if spec == nil {
		return nil, nil
	}
	if len(spec) == 0 {
		return map[string]interface{}{}, nil
	}

	payload, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal MediaWorkflowConfiguration parameters: %w", err)
	}
	var parameters map[string]interface{}
	if err := json.Unmarshal(payload, &parameters); err != nil {
		return nil, fmt.Errorf("decode MediaWorkflowConfiguration parameters: %w", err)
	}
	return parameters, nil
}

func mediaWorkflowConfigurationDefinedTagsFromSpec(spec map[string]shared.MapValue) (map[string]map[string]interface{}, error) {
	if spec == nil {
		return nil, nil
	}
	if len(spec) == 0 {
		return map[string]map[string]interface{}{}, nil
	}

	payload, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal MediaWorkflowConfiguration definedTags: %w", err)
	}
	var tags map[string]map[string]interface{}
	if err := json.Unmarshal(payload, &tags); err != nil {
		return nil, fmt.Errorf("decode MediaWorkflowConfiguration definedTags: %w", err)
	}
	return tags, nil
}

func mediaWorkflowConfigurationCanonicalJSONString(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	switch string(payload) {
	case "", "null", "{}", "[]":
		return "", nil
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	switch string(normalized) {
	case "", "null", "{}", "[]":
		return "", nil
	default:
		return string(normalized), nil
	}
}

func mediaWorkflowConfigurationLocksEqual(spec []mediaservicesv1beta1.MediaWorkflowConfigurationLock, current []mediaservicessdk.ResourceLock) bool {
	if len(spec) != len(current) {
		return false
	}
	for index, lock := range spec {
		if lock.Type != string(current[index].Type) ||
			lock.CompartmentId != mediaWorkflowConfigurationStringValue(current[index].CompartmentId) ||
			lock.RelatedResourceId != mediaWorkflowConfigurationStringValue(current[index].RelatedResourceId) ||
			lock.Message != mediaWorkflowConfigurationStringValue(current[index].Message) {
			return false
		}
	}
	return true
}

func mediaWorkflowConfigurationCanonicalizeDesiredLocks(
	spec []mediaservicesv1beta1.MediaWorkflowConfigurationLock,
	current []mediaservicessdk.ResourceLock,
) ([]mediaservicesv1beta1.MediaWorkflowConfigurationLock, bool) {
	if !mediaWorkflowConfigurationLocksEqual(spec, current) {
		return nil, false
	}

	normalized := append([]mediaservicesv1beta1.MediaWorkflowConfigurationLock(nil), spec...)
	for index := range normalized {
		normalized[index].TimeCreated = mediaWorkflowConfigurationSDKTimeString(current[index].TimeCreated)
	}
	return normalized, true
}

func mediaWorkflowConfigurationStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func mediaWorkflowConfigurationSDKTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.Time.Format(time.RFC3339Nano)
}
