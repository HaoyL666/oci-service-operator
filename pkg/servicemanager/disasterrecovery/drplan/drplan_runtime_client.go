/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drplan

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"unicode"

	"github.com/oracle/oci-go-sdk/v65/common"
	disasterrecoverysdk "github.com/oracle/oci-go-sdk/v65/disasterrecovery"
	disasterrecoveryv1beta1 "github.com/oracle/oci-service-operator/api/disasterrecovery/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

const drPlanKind = "DrPlan"

var drPlanWorkRequestAsyncAdapter = servicemanager.WorkRequestAsyncAdapter{
	PendingStatusTokens: []string{
		string(disasterrecoverysdk.OperationStatusAccepted),
		string(disasterrecoverysdk.OperationStatusInProgress),
		string(disasterrecoverysdk.OperationStatusWaiting),
		string(disasterrecoverysdk.OperationStatusCanceling),
	},
	SucceededStatusTokens: []string{string(disasterrecoverysdk.OperationStatusSucceeded)},
	FailedStatusTokens:    []string{string(disasterrecoverysdk.OperationStatusFailed)},
	CanceledStatusTokens:  []string{string(disasterrecoverysdk.OperationStatusCanceled)},
	AttentionStatusTokens: []string{string(disasterrecoverysdk.OperationStatusNeedsAttention)},
	CreateActionTokens:    []string{string(disasterrecoverysdk.OperationTypeCreateDrPlan)},
	UpdateActionTokens:    []string{string(disasterrecoverysdk.OperationTypeUpdateDrPlan)},
}

type drPlanOCIClient interface {
	CreateDrPlan(context.Context, disasterrecoverysdk.CreateDrPlanRequest) (disasterrecoverysdk.CreateDrPlanResponse, error)
	GetDrPlan(context.Context, disasterrecoverysdk.GetDrPlanRequest) (disasterrecoverysdk.GetDrPlanResponse, error)
	ListDrPlans(context.Context, disasterrecoverysdk.ListDrPlansRequest) (disasterrecoverysdk.ListDrPlansResponse, error)
	UpdateDrPlan(context.Context, disasterrecoverysdk.UpdateDrPlanRequest) (disasterrecoverysdk.UpdateDrPlanResponse, error)
	DeleteDrPlan(context.Context, disasterrecoverysdk.DeleteDrPlanRequest) (disasterrecoverysdk.DeleteDrPlanResponse, error)
	GetWorkRequest(context.Context, disasterrecoverysdk.GetWorkRequestRequest) (disasterrecoverysdk.GetWorkRequestResponse, error)
}

func init() {
	registerDrPlanRuntimeHooksMutator(func(manager *DrPlanServiceManager, hooks *DrPlanRuntimeHooks) {
		client, initErr := newDrPlanSDKClient(manager)
		applyDrPlanRuntimeHooks(hooks, client, initErr)
	})
}

func newDrPlanSDKClient(manager *DrPlanServiceManager) (drPlanOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("%s service manager is nil", drPlanKind)
	}

	client, err := disasterrecoverysdk.NewDisasterRecoveryClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyDrPlanRuntimeHooks(
	hooks *DrPlanRuntimeHooks,
	client drPlanOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedDrPlanRuntimeSemantics()
	hooks.Identity.GuardExistingBeforeCreate = guardDrPlanExistingBeforeCreate
	hooks.List.Fields = drPlanListFields()
	hooks.BuildCreateBody = func(
		_ context.Context,
		resource *disasterrecoveryv1beta1.DrPlan,
		_ string,
	) (any, error) {
		return buildDrPlanCreateBody(resource)
	}
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *disasterrecoveryv1beta1.DrPlan,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildDrPlanUpdateBody(resource, currentResponse)
	}
	hooks.Async.Adapter = drPlanWorkRequestAsyncAdapter
	hooks.Async.GetWorkRequest = func(ctx context.Context, workRequestID string) (any, error) {
		return getDrPlanWorkRequest(ctx, client, initErr, workRequestID)
	}
	hooks.Async.ResolveAction = resolveDrPlanGeneratedWorkRequestAction
	hooks.Async.ResolvePhase = resolveDrPlanGeneratedWorkRequestPhase
	hooks.Async.RecoverResourceID = recoverDrPlanIDFromGeneratedWorkRequest
	hooks.Async.Message = drPlanGeneratedWorkRequestMessage
}

func reviewedDrPlanRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newDrPlanRuntimeSemantics()
	semantics.Lifecycle = generatedruntime.LifecycleSemantics{
		ProvisioningStates: []string{"CREATING"},
		UpdatingStates:     []string{"UPDATING"},
		ActiveStates:       []string{"ACTIVE", "INACTIVE"},
	}
	semantics.List = &generatedruntime.ListSemantics{
		ResponseItemsField: "Items",
		MatchFields:        []string{"drProtectionGroupId", "displayName", "type"},
	}
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable:       []string{"definedTags", "displayName", "freeformTags", "planGroups"},
		ForceNew:      []string{"drProtectionGroupId", "sourcePlanId", "type"},
		ConflictsWith: map[string][]string{},
	}
	semantics.Hooks = generatedruntime.HookSet{
		Create: []generatedruntime.Hook{
			{Helper: "tfresource.CreateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "drplan", Action: "CREATED"},
		},
		Update: []generatedruntime.Hook{
			{Helper: "tfresource.UpdateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "drplan", Action: "UPDATED"},
		},
		Delete: []generatedruntime.Hook{{Helper: "tfresource.DeleteResource"}},
	}
	semantics.CreateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetDrPlan",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Create...),
	}
	semantics.UpdateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetDrPlan",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Update...),
	}
	semantics.DeleteFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetDrPlan/ListDrPlans confirm-delete",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Delete...),
	}
	semantics.AuxiliaryOperations = nil
	return semantics
}

func drPlanListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{
			FieldName:    "DrProtectionGroupId",
			RequestName:  "drProtectionGroupId",
			Contribution: "query",
			LookupPaths:  []string{"status.drProtectionGroupId", "spec.drProtectionGroupId", "drProtectionGroupId"},
		},
		{
			FieldName:    "DisplayName",
			RequestName:  "displayName",
			Contribution: "query",
			LookupPaths:  []string{"status.displayName", "spec.displayName", "displayName"},
		},
		{
			FieldName:    "DrPlanType",
			RequestName:  "drPlanType",
			Contribution: "query",
			LookupPaths:  []string{"status.type", "spec.type", "type"},
		},
		{
			FieldName:    "DrPlanId",
			RequestName:  "drPlanId",
			Contribution: "query",
			LookupPaths:  []string{"status.id", "status.ocid", "id", "ocid"},
		},
	}
}

func guardDrPlanExistingBeforeCreate(
	_ context.Context,
	resource *disasterrecoveryv1beta1.DrPlan,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s resource is nil", drPlanKind)
	}
	if strings.TrimSpace(resource.Spec.DrProtectionGroupId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s spec.drProtectionGroupId is required", drPlanKind)
	}
	if strings.TrimSpace(resource.Spec.DisplayName) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s spec.displayName is required", drPlanKind)
	}
	if strings.TrimSpace(resource.Spec.Type) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s spec.type is required", drPlanKind)
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func buildDrPlanCreateBody(
	resource *disasterrecoveryv1beta1.DrPlan,
) (disasterrecoverysdk.CreateDrPlanDetails, error) {
	if resource == nil {
		return disasterrecoverysdk.CreateDrPlanDetails{}, fmt.Errorf("%s resource is nil", drPlanKind)
	}

	displayName := strings.TrimSpace(resource.Spec.DisplayName)
	if displayName == "" {
		return disasterrecoverysdk.CreateDrPlanDetails{}, fmt.Errorf("%s spec.displayName is required", drPlanKind)
	}
	drProtectionGroupID := strings.TrimSpace(resource.Spec.DrProtectionGroupId)
	if drProtectionGroupID == "" {
		return disasterrecoverysdk.CreateDrPlanDetails{}, fmt.Errorf("%s spec.drProtectionGroupId is required", drPlanKind)
	}
	planType := strings.TrimSpace(resource.Spec.Type)
	if planType == "" {
		return disasterrecoverysdk.CreateDrPlanDetails{}, fmt.Errorf("%s spec.type is required", drPlanKind)
	}

	details := disasterrecoverysdk.CreateDrPlanDetails{
		DisplayName:         common.String(displayName),
		Type:                disasterrecoverysdk.DrPlanTypeEnum(planType),
		DrProtectionGroupId: common.String(drProtectionGroupID),
	}

	if sourcePlanID := strings.TrimSpace(resource.Spec.SourcePlanId); sourcePlanID != "" {
		details.SourcePlanId = common.String(sourcePlanID)
	}
	if resource.Spec.FreeformTags != nil {
		details.FreeformTags = maps.Clone(resource.Spec.FreeformTags)
		if len(details.FreeformTags) == 0 {
			details.FreeformTags = map[string]string{}
		}
	}
	if resource.Spec.DefinedTags != nil {
		details.DefinedTags = drPlanDefinedTagsFromSpec(resource.Spec.DefinedTags)
	}

	return details, nil
}

func buildDrPlanUpdateBody(
	resource *disasterrecoveryv1beta1.DrPlan,
	currentResponse any,
) (disasterrecoverysdk.UpdateDrPlanDetails, bool, error) {
	if resource == nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, fmt.Errorf("%s resource is nil", drPlanKind)
	}

	desiredPayload, err := drPlanUpdatePayload(resource.Spec)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, err
	}

	desiredRaw, err := json.Marshal(desiredPayload)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, fmt.Errorf("marshal desired %s update body: %w", drPlanKind, err)
	}

	var desired disasterrecoverysdk.UpdateDrPlanDetails
	if err := json.Unmarshal(desiredRaw, &desired); err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, fmt.Errorf("decode desired %s update body: %w", drPlanKind, err)
	}

	desiredValues, err := drPlanPrunedJSONMap(desired)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, fmt.Errorf("project desired %s update body: %w", drPlanKind, err)
	}

	current, err := currentDrPlanUpdateDetails(currentResponse)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, err
	}
	currentValues, err := drPlanPrunedJSONMap(current)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, false, fmt.Errorf("project current %s update body: %w", drPlanKind, err)
	}

	normalizeDrPlanUpdateMaps(desiredValues, currentValues)
	return desired, !drPlanMapSubsetEqual(desiredValues, currentValues), nil
}

func drPlanUpdatePayload(
	spec disasterrecoveryv1beta1.DrPlanSpec,
) (map[string]any, error) {
	displayName := strings.TrimSpace(spec.DisplayName)
	if displayName == "" {
		return nil, fmt.Errorf("%s spec.displayName is required", drPlanKind)
	}

	payload := map[string]any{
		"displayName": displayName,
	}

	if spec.PlanGroups != nil {
		planGroups, err := drPlanPlanGroupsPayload(spec.PlanGroups)
		if err != nil {
			return nil, err
		}
		payload["planGroups"] = planGroups
	}
	if spec.FreeformTags != nil {
		payload["freeformTags"] = maps.Clone(spec.FreeformTags)
		if len(spec.FreeformTags) == 0 {
			payload["freeformTags"] = map[string]string{}
		}
	}
	if spec.DefinedTags != nil {
		payload["definedTags"] = drPlanDefinedTagsFromSpec(spec.DefinedTags)
	}

	return payload, nil
}

func drPlanPlanGroupsPayload(
	planGroups []disasterrecoveryv1beta1.DrPlanPlanGroup,
) ([]any, error) {
	payload := make([]any, 0, len(planGroups))
	for _, planGroup := range planGroups {
		groupPayload, err := drPlanPlanGroupPayload(planGroup)
		if err != nil {
			return nil, err
		}
		payload = append(payload, groupPayload)
	}
	return payload, nil
}

func drPlanPlanGroupPayload(
	planGroup disasterrecoveryv1beta1.DrPlanPlanGroup,
) (map[string]any, error) {
	payload := map[string]any{}

	if groupID := strings.TrimSpace(planGroup.Id); groupID != "" {
		payload["id"] = groupID
	}
	if displayName := strings.TrimSpace(planGroup.DisplayName); displayName != "" {
		payload["displayName"] = displayName
	}
	if groupType := strings.TrimSpace(planGroup.Type); groupType != "" {
		payload["type"] = groupType
	}
	if strings.EqualFold(strings.TrimSpace(planGroup.Type), "USER_DEFINED_PAUSE") {
		payload["isPauseEnabled"] = planGroup.IsPauseEnabled
	}
	if planGroup.Steps != nil {
		stepsPayload, err := drPlanPlanStepsPayload(planGroup.Steps)
		if err != nil {
			return nil, err
		}
		payload["steps"] = stepsPayload
	}

	return payload, nil
}

func drPlanPlanStepsPayload(
	planSteps []disasterrecoveryv1beta1.DrPlanPlanGroupStep,
) ([]any, error) {
	payload := make([]any, 0, len(planSteps))
	for _, planStep := range planSteps {
		stepPayload, err := drPlanPlanStepPayload(planStep)
		if err != nil {
			return nil, err
		}
		payload = append(payload, stepPayload)
	}
	return payload, nil
}

func drPlanPlanStepPayload(
	planStep disasterrecoveryv1beta1.DrPlanPlanGroupStep,
) (map[string]any, error) {
	payload := map[string]any{
		"isEnabled": planStep.IsEnabled,
	}

	if stepID := strings.TrimSpace(planStep.Id); stepID != "" {
		payload["id"] = stepID
	}
	if displayName := strings.TrimSpace(planStep.DisplayName); displayName != "" {
		payload["displayName"] = displayName
	}
	if errorMode := strings.TrimSpace(planStep.ErrorMode); errorMode != "" {
		payload["errorMode"] = errorMode
	}
	if planStep.Timeout != 0 {
		payload["timeout"] = planStep.Timeout
	}
	if userDefinedStep, ok, err := drPlanUserDefinedStepPayload(planStep.UserDefinedStep); err != nil {
		return nil, err
	} else if ok {
		payload["userDefinedStep"] = userDefinedStep
	}

	return payload, nil
}

func drPlanUserDefinedStepPayload(
	userDefinedStep disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStep,
) (map[string]any, bool, error) {
	payload := map[string]any{}

	stepType := strings.TrimSpace(userDefinedStep.StepType)
	if stepType != "" {
		payload["stepType"] = stepType
	}
	if jsonData := strings.TrimSpace(userDefinedStep.JsonData); jsonData != "" {
		payload["jsonData"] = jsonData
	}
	if runOnInstanceID := strings.TrimSpace(userDefinedStep.RunOnInstanceId); runOnInstanceID != "" {
		payload["runOnInstanceId"] = runOnInstanceID
	}
	if scriptCommand := strings.TrimSpace(userDefinedStep.ScriptCommand); scriptCommand != "" {
		payload["scriptCommand"] = scriptCommand
	}
	if runAsUser := strings.TrimSpace(userDefinedStep.RunAsUser); runAsUser != "" {
		payload["runAsUser"] = runAsUser
	}
	if functionID := strings.TrimSpace(userDefinedStep.FunctionId); functionID != "" {
		payload["functionId"] = functionID
	}
	if requestBody := strings.TrimSpace(userDefinedStep.RequestBody); requestBody != "" {
		payload["requestBody"] = requestBody
	}
	if objectStorageScriptLocation, ok, err := drPlanObjectStorageScriptLocationPayload(userDefinedStep.ObjectStorageScriptLocation); err != nil {
		return nil, false, err
	} else if ok {
		payload["objectStorageScriptLocation"] = objectStorageScriptLocation
	}

	if len(payload) == 0 {
		return nil, false, nil
	}
	if stepType == "" {
		return nil, false, fmt.Errorf("%s planGroups.steps.userDefinedStep.stepType is required when userDefinedStep fields are set", drPlanKind)
	}

	return payload, true, nil
}

func drPlanObjectStorageScriptLocationPayload(
	location disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStepObjectStorageScriptLocation,
) (map[string]any, bool, error) {
	namespace := strings.TrimSpace(location.Namespace)
	bucket := strings.TrimSpace(location.Bucket)
	object := strings.TrimSpace(location.Object)

	if namespace == "" && bucket == "" && object == "" {
		return nil, false, nil
	}
	if namespace == "" || bucket == "" || object == "" {
		return nil, false, fmt.Errorf("%s userDefinedStep.objectStorageScriptLocation requires namespace, bucket, and object", drPlanKind)
	}

	return map[string]any{
		"namespace": namespace,
		"bucket":    bucket,
		"object":    object,
	}, true, nil
}

func currentDrPlanUpdateDetails(
	currentResponse any,
) (disasterrecoverysdk.UpdateDrPlanDetails, error) {
	if currentResponse == nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, nil
	}

	body, err := drPlanRuntimeBody(currentResponse)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, err
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, fmt.Errorf("marshal current %s response: %w", drPlanKind, err)
	}

	var details disasterrecoverysdk.UpdateDrPlanDetails
	if err := json.Unmarshal(raw, &details); err != nil {
		return disasterrecoverysdk.UpdateDrPlanDetails{}, fmt.Errorf("decode current %s update body: %w", drPlanKind, err)
	}
	return details, nil
}

func drPlanRuntimeBody(
	currentResponse any,
) (disasterrecoverysdk.DrPlan, error) {
	switch current := currentResponse.(type) {
	case disasterrecoverysdk.DrPlan:
		return current, nil
	case *disasterrecoverysdk.DrPlan:
		if current == nil {
			return disasterrecoverysdk.DrPlan{}, fmt.Errorf("current %s response is nil", drPlanKind)
		}
		return *current, nil
	case disasterrecoverysdk.DrPlanSummary:
		return drPlanFromSummary(current), nil
	case *disasterrecoverysdk.DrPlanSummary:
		if current == nil {
			return disasterrecoverysdk.DrPlan{}, fmt.Errorf("current %s response is nil", drPlanKind)
		}
		return drPlanFromSummary(*current), nil
	case disasterrecoverysdk.CreateDrPlanResponse:
		return current.DrPlan, nil
	case *disasterrecoverysdk.CreateDrPlanResponse:
		if current == nil {
			return disasterrecoverysdk.DrPlan{}, fmt.Errorf("current %s response is nil", drPlanKind)
		}
		return current.DrPlan, nil
	case disasterrecoverysdk.GetDrPlanResponse:
		return current.DrPlan, nil
	case *disasterrecoverysdk.GetDrPlanResponse:
		if current == nil {
			return disasterrecoverysdk.DrPlan{}, fmt.Errorf("current %s response is nil", drPlanKind)
		}
		return current.DrPlan, nil
	default:
		return disasterrecoverysdk.DrPlan{}, fmt.Errorf("unexpected %s response type %T", drPlanKind, currentResponse)
	}
}

func drPlanFromSummary(summary disasterrecoverysdk.DrPlanSummary) disasterrecoverysdk.DrPlan {
	return disasterrecoverysdk.DrPlan{
		Id:                      summary.Id,
		CompartmentId:           summary.CompartmentId,
		DisplayName:             summary.DisplayName,
		Type:                    summary.Type,
		TimeCreated:             summary.TimeCreated,
		TimeUpdated:             summary.TimeUpdated,
		DrProtectionGroupId:     summary.DrProtectionGroupId,
		PeerDrProtectionGroupId: summary.PeerDrProtectionGroupId,
		PeerRegion:              summary.PeerRegion,
		LifecycleState:          summary.LifecycleState,
		LifecycleSubState:       summary.LifecycleSubState,
		LifeCycleDetails:        summary.LifeCycleDetails,
		FreeformTags:            summary.FreeformTags,
		DefinedTags:             summary.DefinedTags,
		SystemTags:              summary.SystemTags,
	}
}

func getDrPlanWorkRequest(
	ctx context.Context,
	client drPlanOCIClient,
	initErr error,
	workRequestID string,
) (any, error) {
	if initErr != nil {
		return nil, fmt.Errorf("initialize %s OCI client: %w", drPlanKind, initErr)
	}
	if client == nil {
		return nil, fmt.Errorf("%s OCI client is not configured", drPlanKind)
	}

	response, err := client.GetWorkRequest(ctx, disasterrecoverysdk.GetWorkRequestRequest{
		WorkRequestId: common.String(strings.TrimSpace(workRequestID)),
	})
	if err != nil {
		return nil, err
	}
	return response.WorkRequest, nil
}

func resolveDrPlanGeneratedWorkRequestAction(workRequest any) (string, error) {
	current, err := drPlanWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}
	return string(current.OperationType), nil
}

func resolveDrPlanGeneratedWorkRequestPhase(workRequest any) (shared.OSOKAsyncPhase, bool, error) {
	current, err := drPlanWorkRequestFromAny(workRequest)
	if err != nil {
		return "", false, err
	}
	phase, ok := drPlanWorkRequestPhaseFromOperationType(current.OperationType)
	return phase, ok, nil
}

func recoverDrPlanIDFromGeneratedWorkRequest(
	_ *disasterrecoveryv1beta1.DrPlan,
	workRequest any,
	phase shared.OSOKAsyncPhase,
) (string, error) {
	current, err := drPlanWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}

	action := drPlanWorkRequestActionForPhase(phase)
	if id, ok := resolveDrPlanIDFromWorkRequestResources(current.Resources, action, true); ok {
		return id, nil
	}
	if id, ok := resolveDrPlanIDFromWorkRequestResources(current.Resources, action, false); ok {
		return id, nil
	}
	return "", fmt.Errorf("%s %s work request %s did not expose a %s identifier", drPlanKind, phase, drPlanStringValue(current.Id), drPlanKind)
}

func drPlanGeneratedWorkRequestMessage(phase shared.OSOKAsyncPhase, workRequest any) string {
	current, err := drPlanWorkRequestFromAny(workRequest)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s %s work request %s is %s", drPlanKind, phase, drPlanStringValue(current.Id), current.Status)
}

func drPlanWorkRequestFromAny(workRequest any) (disasterrecoverysdk.WorkRequest, error) {
	switch current := workRequest.(type) {
	case disasterrecoverysdk.WorkRequest:
		return current, nil
	case *disasterrecoverysdk.WorkRequest:
		if current == nil {
			return disasterrecoverysdk.WorkRequest{}, fmt.Errorf("%s work request is nil", drPlanKind)
		}
		return *current, nil
	default:
		return disasterrecoverysdk.WorkRequest{}, fmt.Errorf("unexpected %s work request type %T", drPlanKind, workRequest)
	}
}

func drPlanWorkRequestPhaseFromOperationType(
	operationType disasterrecoverysdk.OperationTypeEnum,
) (shared.OSOKAsyncPhase, bool) {
	switch operationType {
	case disasterrecoverysdk.OperationTypeCreateDrPlan:
		return shared.OSOKAsyncPhaseCreate, true
	case disasterrecoverysdk.OperationTypeUpdateDrPlan:
		return shared.OSOKAsyncPhaseUpdate, true
	default:
		return "", false
	}
}

func drPlanWorkRequestActionForPhase(phase shared.OSOKAsyncPhase) disasterrecoverysdk.ActionTypeEnum {
	switch phase {
	case shared.OSOKAsyncPhaseCreate:
		return disasterrecoverysdk.ActionTypeCreated
	case shared.OSOKAsyncPhaseUpdate:
		return disasterrecoverysdk.ActionTypeUpdated
	default:
		return ""
	}
}

func resolveDrPlanIDFromWorkRequestResources(
	resources []disasterrecoverysdk.WorkRequestResource,
	action disasterrecoverysdk.ActionTypeEnum,
	preferDrPlanOnly bool,
) (string, bool) {
	var candidate string
	for _, resource := range resources {
		if action != "" && resource.ActionType != action {
			continue
		}
		if preferDrPlanOnly && !isDrPlanWorkRequestResource(resource) {
			continue
		}

		id := strings.TrimSpace(drPlanStringValue(resource.Identifier))
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

func isDrPlanWorkRequestResource(resource disasterrecoverysdk.WorkRequestResource) bool {
	entityToken := normalizeDrPlanWorkRequestToken(drPlanStringValue(resource.EntityType))
	if entityToken == "drplan" || strings.Contains(entityToken, "drplan") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(drPlanStringValue(resource.Identifier))), "ocid1.drplan")
}

func normalizeDrPlanWorkRequestToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func drPlanDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
	if spec == nil {
		return nil
	}

	var out map[string]map[string]interface{}
	if err := drPlanJSONConvert(spec, &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return map[string]map[string]interface{}{}
	}
	return out
}

func drPlanJSONMap(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func drPlanPrunedJSONMap(value any) (map[string]any, error) {
	values, err := drPlanJSONMap(value)
	if err != nil {
		return nil, err
	}

	pruned, ok := drPlanPruneJSONValue(values)
	if !ok {
		return map[string]any{}, nil
	}

	prunedMap, ok := pruned.(map[string]any)
	if !ok || prunedMap == nil {
		return map[string]any{}, nil
	}
	return prunedMap, nil
}

func drPlanPruneJSONValue(value any) (any, bool) {
	switch current := value.(type) {
	case nil:
		return nil, false
	case map[string]any:
		if len(current) == 0 {
			return map[string]any{}, true
		}

		pruned := make(map[string]any, len(current))
		for key, child := range current {
			prunedChild, ok := drPlanPruneJSONValue(child)
			if !ok {
				continue
			}
			pruned[key] = prunedChild
		}
		if len(pruned) == 0 {
			return nil, false
		}
		return pruned, true
	case []any:
		if len(current) == 0 {
			return []any{}, true
		}

		pruned := make([]any, 0, len(current))
		for _, child := range current {
			prunedChild, ok := drPlanPruneJSONValue(child)
			if !ok {
				continue
			}
			pruned = append(pruned, prunedChild)
		}
		if len(pruned) == 0 {
			return []any{}, true
		}
		return pruned, true
	default:
		return value, true
	}
}

func normalizeDrPlanUpdateMaps(desired map[string]any, current map[string]any) {
	normalizeDrPlanEmptyContainer(desired, current, "planGroups")
	normalizeDrPlanEmptyContainer(desired, current, "freeformTags")
	normalizeDrPlanEmptyContainer(desired, current, "definedTags")
}

func normalizeDrPlanEmptyContainer(desired map[string]any, current map[string]any, key string) {
	desiredValue, ok := desired[key]
	if !ok {
		return
	}

	switch desiredValue.(type) {
	case []any:
		if _, ok := current[key]; !ok || current[key] == nil {
			current[key] = []any{}
		}
	case map[string]any:
		if _, ok := current[key]; !ok || current[key] == nil {
			current[key] = map[string]any{}
		}
	}
}

func drPlanJSONConvert(source any, destination any) error {
	payload, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, destination)
}

func drPlanMapSubsetEqual(want map[string]any, got map[string]any) bool {
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return false
		}
		if !drPlanJSONValueEqual(wantValue, gotValue) {
			return false
		}
	}
	return true
}

func drPlanJSONValueEqual(left any, right any) bool {
	leftMap, leftIsMap := left.(map[string]any)
	rightMap, rightIsMap := right.(map[string]any)
	switch {
	case leftIsMap && rightIsMap:
		return drPlanMapSubsetEqual(leftMap, rightMap)
	case leftIsMap || rightIsMap:
		return false
	}

	leftSlice, leftIsSlice := left.([]any)
	rightSlice, rightIsSlice := right.([]any)
	switch {
	case leftIsSlice && rightIsSlice:
		if len(leftSlice) != len(rightSlice) {
			return false
		}
		for i := range leftSlice {
			if !drPlanJSONValueEqual(leftSlice[i], rightSlice[i]) {
				return false
			}
		}
		return true
	case leftIsSlice || rightIsSlice:
		return false
	}

	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func drPlanStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
