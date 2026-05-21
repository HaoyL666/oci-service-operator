/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
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

var dataSourceWorkRequestAsyncAdapter = servicemanager.WorkRequestAsyncAdapter{
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
	CreateActionTokens:    []string{string(generativeaiagentsdk.OperationTypeCreateDataSource)},
	UpdateActionTokens:    []string{string(generativeaiagentsdk.OperationTypeUpdateDataSource)},
	DeleteActionTokens:    []string{string(generativeaiagentsdk.OperationTypeDeleteDataSource)},
}

type dataSourceOCIClient interface {
	CreateDataSource(context.Context, generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error)
	GetDataSource(context.Context, generativeaiagentsdk.GetDataSourceRequest) (generativeaiagentsdk.GetDataSourceResponse, error)
	ListDataSources(context.Context, generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error)
	UpdateDataSource(context.Context, generativeaiagentsdk.UpdateDataSourceRequest) (generativeaiagentsdk.UpdateDataSourceResponse, error)
	DeleteDataSource(context.Context, generativeaiagentsdk.DeleteDataSourceRequest) (generativeaiagentsdk.DeleteDataSourceResponse, error)
	GetWorkRequest(context.Context, generativeaiagentsdk.GetWorkRequestRequest) (generativeaiagentsdk.GetWorkRequestResponse, error)
}

type projectedDataSourceConfig struct {
	JsonData                  string           `json:"jsonData,omitempty"`
	ShouldEnableMultiModality *bool            `json:"shouldEnableMultiModality,omitempty"`
	DataSourceConfigType      string           `json:"dataSourceConfigType,omitempty"`
	ObjectStoragePrefixes     []map[string]any `json:"objectStoragePrefixes,omitempty"`
}

func init() {
	registerDataSourceRuntimeHooksMutator(func(manager *DataSourceServiceManager, hooks *DataSourceRuntimeHooks) {
		client, initErr := newDataSourceSDKClient(manager)
		applyDataSourceRuntimeHooks(hooks, client, initErr)
	})
}

func newDataSourceSDKClient(manager *DataSourceServiceManager) (dataSourceOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("DataSource service manager is nil")
	}
	client, err := generativeaiagentsdk.NewGenerativeAiAgentClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyDataSourceRuntimeHooks(
	hooks *DataSourceRuntimeHooks,
	client dataSourceOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedDataSourceRuntimeSemantics()
	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *generativeaiagentv1beta1.DataSource,
		namespace string,
	) (any, error) {
		return buildDataSourceCreateDetails(ctx, resource, namespace)
	}
	hooks.BuildUpdateBody = func(
		ctx context.Context,
		resource *generativeaiagentv1beta1.DataSource,
		namespace string,
		currentResponse any,
	) (any, bool, error) {
		return buildDataSourceUpdateBody(ctx, resource, namespace, currentResponse)
	}
	hooks.Identity.GuardExistingBeforeCreate = guardDataSourceExistingBeforeCreate
	hooks.Create.Fields = dataSourceCreateFields()
	hooks.Get.Fields = dataSourceGetFields()
	hooks.List.Fields = dataSourceListFields()
	hooks.Update.Fields = dataSourceUpdateFields()
	hooks.Delete.Fields = dataSourceDeleteFields()
	hooks.Async.Adapter = dataSourceWorkRequestAsyncAdapter
	hooks.Async.GetWorkRequest = func(ctx context.Context, workRequestID string) (any, error) {
		return getDataSourceWorkRequest(ctx, client, initErr, workRequestID)
	}
	hooks.Async.ResolveAction = resolveDataSourceGeneratedWorkRequestAction
	hooks.Async.ResolvePhase = resolveDataSourceGeneratedWorkRequestPhase
	hooks.Async.RecoverResourceID = recoverDataSourceIDFromGeneratedWorkRequest
	hooks.Async.Message = dataSourceGeneratedWorkRequestMessage
}

func newDataSourceServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client dataSourceOCIClient,
) DataSourceServiceClient {
	return defaultDataSourceServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*generativeaiagentv1beta1.DataSource](
			newDataSourceRuntimeConfig(log, client),
		),
	}
}

func newDataSourceRuntimeConfig(
	log loggerutil.OSOKLogger,
	client dataSourceOCIClient,
) generatedruntime.Config[*generativeaiagentv1beta1.DataSource] {
	hooks := newDataSourceRuntimeHooksWithOCIClient(client)
	applyDataSourceRuntimeHooks(&hooks, client, nil)
	return buildDataSourceGeneratedRuntimeConfig(&DataSourceServiceManager{Log: log}, hooks)
}

func newDataSourceRuntimeHooksWithOCIClient(client dataSourceOCIClient) DataSourceRuntimeHooks {
	return DataSourceRuntimeHooks{
		Semantics: newDataSourceRuntimeSemantics(),
		Create: runtimeOperationHooks[generativeaiagentsdk.CreateDataSourceRequest, generativeaiagentsdk.CreateDataSourceResponse]{
			Fields: dataSourceCreateFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.CreateDataSourceRequest) (generativeaiagentsdk.CreateDataSourceResponse, error) {
				return client.CreateDataSource(ctx, request)
			},
		},
		Get: runtimeOperationHooks[generativeaiagentsdk.GetDataSourceRequest, generativeaiagentsdk.GetDataSourceResponse]{
			Fields: dataSourceGetFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.GetDataSourceRequest) (generativeaiagentsdk.GetDataSourceResponse, error) {
				return client.GetDataSource(ctx, request)
			},
		},
		List: runtimeOperationHooks[generativeaiagentsdk.ListDataSourcesRequest, generativeaiagentsdk.ListDataSourcesResponse]{
			Fields: dataSourceListFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.ListDataSourcesRequest) (generativeaiagentsdk.ListDataSourcesResponse, error) {
				return client.ListDataSources(ctx, request)
			},
		},
		Update: runtimeOperationHooks[generativeaiagentsdk.UpdateDataSourceRequest, generativeaiagentsdk.UpdateDataSourceResponse]{
			Fields: dataSourceUpdateFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.UpdateDataSourceRequest) (generativeaiagentsdk.UpdateDataSourceResponse, error) {
				return client.UpdateDataSource(ctx, request)
			},
		},
		Delete: runtimeOperationHooks[generativeaiagentsdk.DeleteDataSourceRequest, generativeaiagentsdk.DeleteDataSourceResponse]{
			Fields: dataSourceDeleteFields(),
			Call: func(ctx context.Context, request generativeaiagentsdk.DeleteDataSourceRequest) (generativeaiagentsdk.DeleteDataSourceResponse, error) {
				return client.DeleteDataSource(ctx, request)
			},
		},
	}
}

func reviewedDataSourceRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newDataSourceRuntimeSemantics()
	semantics.Lifecycle = generatedruntime.LifecycleSemantics{
		ProvisioningStates: []string{"CREATING"},
		UpdatingStates:     []string{"UPDATING"},
		ActiveStates:       []string{"ACTIVE", "INACTIVE"},
	}
	semantics.Delete = generatedruntime.DeleteSemantics{
		Policy:         "required",
		PendingStates:  []string{"DELETING"},
		TerminalStates: []string{"DELETED"},
	}
	semantics.List = &generatedruntime.ListSemantics{
		ResponseItemsField: "Items",
		MatchFields:        []string{"compartmentId", "knowledgeBaseId", "displayName"},
	}
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
			"dataSourceConfig",
			"definedTags",
			"description",
			"displayName",
			"freeformTags",
			"metadata",
		},
		ForceNew:      []string{"compartmentId", "knowledgeBaseId"},
		ConflictsWith: map[string][]string{},
	}
	semantics.Hooks = generatedruntime.HookSet{
		Create: []generatedruntime.Hook{
			{Helper: "tfresource.CreateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "datasource", Action: "CREATED"},
		},
		Update: []generatedruntime.Hook{
			{Helper: "tfresource.UpdateResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "datasource", Action: "UPDATED"},
		},
		Delete: []generatedruntime.Hook{
			{Helper: "tfresource.DeleteResource"},
			{Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "datasource", Action: "DELETED"},
		},
	}
	semantics.CreateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetDataSource",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Create...),
	}
	semantics.UpdateFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetDataSource",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Update...),
	}
	semantics.DeleteFollowUp = generatedruntime.FollowUpSemantics{
		Strategy: "GetWorkRequest -> GetDataSource/ListDataSources confirm-delete",
		Hooks:    append([]generatedruntime.Hook(nil), semantics.Hooks.Delete...),
	}
	semantics.AuxiliaryOperations = nil
	return semantics
}

func dataSourceCreateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CreateDataSourceDetails", RequestName: "CreateDataSourceDetails", Contribution: "body"},
	}
}

func dataSourceGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "DataSourceId", RequestName: "dataSourceId", Contribution: "path", PreferResourceID: true},
	}
}

func dataSourceListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{
			FieldName:    "CompartmentId",
			RequestName:  "compartmentId",
			Contribution: "query",
			LookupPaths:  []string{"status.compartmentId", "spec.compartmentId", "compartmentId"},
		},
		{
			FieldName:    "KnowledgeBaseId",
			RequestName:  "knowledgeBaseId",
			Contribution: "query",
			LookupPaths:  []string{"status.knowledgeBaseId", "spec.knowledgeBaseId", "knowledgeBaseId"},
		},
		{
			FieldName:    "DisplayName",
			RequestName:  "displayName",
			Contribution: "query",
			LookupPaths:  []string{"status.displayName", "spec.displayName", "displayName"},
		},
	}
}

func dataSourceUpdateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "DataSourceId", RequestName: "dataSourceId", Contribution: "path", PreferResourceID: true},
		{FieldName: "UpdateDataSourceDetails", RequestName: "UpdateDataSourceDetails", Contribution: "body"},
	}
}

func dataSourceDeleteFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "DataSourceId", RequestName: "dataSourceId", Contribution: "path", PreferResourceID: true},
	}
}

func guardDataSourceExistingBeforeCreate(
	_ context.Context,
	resource *generativeaiagentv1beta1.DataSource,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("DataSource resource is nil")
	}
	if strings.TrimSpace(resource.Spec.DisplayName) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func buildDataSourceCreateDetails(
	ctx context.Context,
	resource *generativeaiagentv1beta1.DataSource,
	namespace string,
) (generativeaiagentsdk.CreateDataSourceDetails, error) {
	if resource == nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, fmt.Errorf("DataSource resource is nil")
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, nil, namespace)
	if err != nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, err
	}

	resolvedSpecMap, err := dataSourceResolvedSpecMap(resolvedSpec)
	if err != nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, err
	}

	config, err := buildDataSourceConfigFromResolvedValue(resolvedSpecMap["dataSourceConfig"])
	if err != nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, err
	}

	payload, err := dataSourceResolvedPayloadWithoutConfig(resolvedSpecMap)
	if err != nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, err
	}

	var details generativeaiagentsdk.CreateDataSourceDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, fmt.Errorf("decode DataSource create request body: %w", err)
	}
	if config == nil {
		return generativeaiagentsdk.CreateDataSourceDetails{}, fmt.Errorf("DataSource dataSourceConfig must resolve to a concrete SDK body")
	}
	details.DataSourceConfig = config

	return details, nil
}

func buildDataSourceUpdateBody(
	ctx context.Context,
	resource *generativeaiagentv1beta1.DataSource,
	namespace string,
	currentResponse any,
) (generativeaiagentsdk.UpdateDataSourceDetails, bool, error) {
	if resource == nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, false, fmt.Errorf("DataSource resource is nil")
	}

	current, err := dataSourceFromResponse(currentResponse)
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, false, err
	}

	desired, err := buildDataSourceDesiredUpdateDetails(ctx, resource, namespace)
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, false, err
	}

	details := generativeaiagentsdk.UpdateDataSourceDetails{}
	updateNeeded := false

	if value, ok := dataSourceDesiredStringUpdate(resource.Spec.DisplayName, current.DisplayName); ok {
		details.DisplayName = value
		updateNeeded = true
	}
	if value, ok := dataSourceDesiredStringUpdate(resource.Spec.Description, current.Description); ok {
		details.Description = value
		updateNeeded = true
	}
	if value, ok := dataSourceDesiredDataSourceConfigUpdate(desired.DataSourceConfig, current.DataSourceConfig); ok {
		details.DataSourceConfig = value
		updateNeeded = true
	}
	if value, ok := dataSourceDesiredMetadataUpdate(resource.Spec.Metadata, current.Metadata); ok {
		details.Metadata = value
		updateNeeded = true
	}
	if value, ok := dataSourceDesiredFreeformTagsUpdate(resource.Spec.FreeformTags, current.FreeformTags); ok {
		details.FreeformTags = value
		updateNeeded = true
	}
	if value, ok := dataSourceDesiredDefinedTagsUpdate(resource.Spec.DefinedTags, current.DefinedTags); ok {
		details.DefinedTags = value
		updateNeeded = true
	}

	return details, updateNeeded, nil
}

func buildDataSourceDesiredUpdateDetails(
	ctx context.Context,
	resource *generativeaiagentv1beta1.DataSource,
	namespace string,
) (generativeaiagentsdk.UpdateDataSourceDetails, error) {
	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, nil, namespace)
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, err
	}

	resolvedSpecMap, err := dataSourceResolvedSpecMap(resolvedSpec)
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, err
	}

	config, err := buildDataSourceConfigFromResolvedValue(resolvedSpecMap["dataSourceConfig"])
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, err
	}

	payload, err := dataSourceResolvedPayloadWithoutConfig(resolvedSpecMap)
	if err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, err
	}

	var details generativeaiagentsdk.UpdateDataSourceDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, fmt.Errorf("decode DataSource update request body: %w", err)
	}
	if config == nil {
		return generativeaiagentsdk.UpdateDataSourceDetails{}, fmt.Errorf("DataSource dataSourceConfig must resolve to a concrete SDK body")
	}
	details.DataSourceConfig = config

	return details, nil
}

func dataSourceResolvedSpecMap(resolvedSpec any) (map[string]any, error) {
	if resolvedSpec == nil {
		return nil, fmt.Errorf("resolved DataSource spec is nil")
	}
	resolvedSpecMap, ok := resolvedSpec.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("resolved DataSource spec is %T, want map[string]any", resolvedSpec)
	}
	return resolvedSpecMap, nil
}

func dataSourceResolvedPayloadWithoutConfig(resolvedSpecMap map[string]any) ([]byte, error) {
	bodySpec := maps.Clone(resolvedSpecMap)
	delete(bodySpec, "dataSourceConfig")

	payload, err := json.Marshal(bodySpec)
	if err != nil {
		return nil, fmt.Errorf("marshal resolved DataSource spec: %w", err)
	}
	return payload, nil
}

func buildDataSourceConfigFromResolvedValue(value any) (generativeaiagentsdk.DataSourceConfig, error) {
	if value == nil {
		return nil, nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal DataSource dataSourceConfig: %w", err)
	}

	var projected projectedDataSourceConfig
	if err := json.Unmarshal(payload, &projected); err != nil {
		return nil, fmt.Errorf("decode DataSource dataSourceConfig projection: %w", err)
	}

	if raw := strings.TrimSpace(projected.JsonData); raw != "" {
		conflicts := dataSourceConfigConflicts(projected)
		if len(conflicts) > 0 {
			return nil, fmt.Errorf("DataSource dataSourceConfig.jsonData conflicts with typed field(s): %s", strings.Join(conflicts, ", "))
		}
		return buildDataSourceConfigFromPayload([]byte(raw))
	}

	return buildDataSourceConfigFromPayload(payload)
}

func dataSourceConfigConflicts(projected projectedDataSourceConfig) []string {
	var conflicts []string
	if strings.TrimSpace(projected.DataSourceConfigType) != "" {
		conflicts = append(conflicts, "dataSourceConfigType")
	}
	if projected.ShouldEnableMultiModality != nil && *projected.ShouldEnableMultiModality {
		conflicts = append(conflicts, "shouldEnableMultiModality")
	}
	if len(projected.ObjectStoragePrefixes) > 0 {
		conflicts = append(conflicts, "objectStoragePrefixes")
	}
	sort.Strings(conflicts)
	return conflicts
}

func buildDataSourceConfigFromPayload(payload []byte) (generativeaiagentsdk.DataSourceConfig, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return nil, nil
	}

	dataSourceConfigType, err := dataSourceJSONFieldString(payload, "dataSourceConfigType")
	if err != nil {
		return nil, fmt.Errorf("decode DataSource dataSourceConfigType: %w", err)
	}

	switch strings.ToUpper(strings.TrimSpace(dataSourceConfigType)) {
	case string(generativeaiagentsdk.DataSourceConfigDataSourceConfigTypeOciObjectStorage):
		var config generativeaiagentsdk.OciObjectStorageDataSourceConfig
		if err := json.Unmarshal(payload, &config); err != nil {
			return nil, fmt.Errorf("decode DataSource OCI_OBJECT_STORAGE config: %w", err)
		}
		return config, nil
	case "":
		return nil, fmt.Errorf("DataSource dataSourceConfigType is required")
	default:
		return nil, fmt.Errorf("unsupported DataSource dataSourceConfigType %q", dataSourceConfigType)
	}
}

func dataSourceJSONFieldString(payload []byte, key string) (string, error) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	raw, ok := decoded[key]
	if !ok || raw == nil {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s is %T, want string", key, raw)
	}
	return value, nil
}

func dataSourceFromResponse(currentResponse any) (generativeaiagentsdk.DataSource, error) {
	switch current := currentResponse.(type) {
	case generativeaiagentsdk.DataSource:
		return current, nil
	case *generativeaiagentsdk.DataSource:
		if current == nil {
			return generativeaiagentsdk.DataSource{}, fmt.Errorf("current DataSource response is nil")
		}
		return *current, nil
	case generativeaiagentsdk.DataSourceSummary:
		return generativeaiagentsdk.DataSource{
			Id:               current.Id,
			DisplayName:      current.DisplayName,
			CompartmentId:    current.CompartmentId,
			KnowledgeBaseId:  current.KnowledgeBaseId,
			TimeCreated:      current.TimeCreated,
			LifecycleState:   current.LifecycleState,
			FreeformTags:     current.FreeformTags,
			DefinedTags:      current.DefinedTags,
			Description:      current.Description,
			TimeUpdated:      current.TimeUpdated,
			LifecycleDetails: current.LifecycleDetails,
			SystemTags:       current.SystemTags,
		}, nil
	case *generativeaiagentsdk.DataSourceSummary:
		if current == nil {
			return generativeaiagentsdk.DataSource{}, fmt.Errorf("current DataSource response is nil")
		}
		return dataSourceFromResponse(*current)
	case generativeaiagentsdk.GetDataSourceResponse:
		return current.DataSource, nil
	case *generativeaiagentsdk.GetDataSourceResponse:
		if current == nil {
			return generativeaiagentsdk.DataSource{}, fmt.Errorf("current DataSource response is nil")
		}
		return current.DataSource, nil
	default:
		return generativeaiagentsdk.DataSource{}, fmt.Errorf("unexpected current DataSource response type %T", currentResponse)
	}
}

func dataSourceDesiredStringUpdate(spec string, current *string) (*string, bool) {
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

func dataSourceDesiredDataSourceConfigUpdate(
	desired generativeaiagentsdk.DataSourceConfig,
	current generativeaiagentsdk.DataSourceConfig,
) (generativeaiagentsdk.DataSourceConfig, bool) {
	if desired == nil {
		return nil, false
	}
	if current == nil {
		return desired, true
	}
	if dataSourceJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func dataSourceDesiredMetadataUpdate(
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

func dataSourceDesiredFreeformTagsUpdate(
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

func dataSourceDesiredDefinedTagsUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) (map[string]map[string]interface{}, bool) {
	if spec == nil {
		return nil, false
	}

	desired := dataSourceDefinedTagsFromSpec(spec)
	if len(desired) == 0 && len(current) == 0 {
		return nil, false
	}
	if dataSourceJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func dataSourceDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
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

func dataSourceJSONEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func getDataSourceWorkRequest(
	ctx context.Context,
	client dataSourceOCIClient,
	initErr error,
	workRequestID string,
) (any, error) {
	if initErr != nil {
		return nil, fmt.Errorf("initialize DataSource OCI client: %w", initErr)
	}
	if client == nil {
		return nil, fmt.Errorf("DataSource OCI client is not configured")
	}

	response, err := client.GetWorkRequest(ctx, generativeaiagentsdk.GetWorkRequestRequest{
		WorkRequestId: common.String(strings.TrimSpace(workRequestID)),
	})
	if err != nil {
		return nil, err
	}
	return response.WorkRequest, nil
}

func resolveDataSourceGeneratedWorkRequestAction(workRequest any) (string, error) {
	current, err := dataSourceWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}
	return string(current.OperationType), nil
}

func resolveDataSourceGeneratedWorkRequestPhase(workRequest any) (shared.OSOKAsyncPhase, bool, error) {
	current, err := dataSourceWorkRequestFromAny(workRequest)
	if err != nil {
		return "", false, err
	}
	phase, ok := dataSourceWorkRequestPhaseFromOperationType(current.OperationType)
	return phase, ok, nil
}

func recoverDataSourceIDFromGeneratedWorkRequest(
	_ *generativeaiagentv1beta1.DataSource,
	workRequest any,
	phase shared.OSOKAsyncPhase,
) (string, error) {
	current, err := dataSourceWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}

	action := dataSourceWorkRequestActionForPhase(phase)
	if id, ok := resolveDataSourceIDFromResources(current.Resources, action, true); ok {
		return id, nil
	}
	if id, ok := resolveDataSourceIDFromResources(current.Resources, action, false); ok {
		return id, nil
	}
	return "", fmt.Errorf("DataSource work request %s does not expose a data source identifier", stringValue(current.Id))
}

func dataSourceGeneratedWorkRequestMessage(phase shared.OSOKAsyncPhase, workRequest any) string {
	current, err := dataSourceWorkRequestFromAny(workRequest)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("DataSource %s work request %s is %s", phase, stringValue(current.Id), current.Status)
}

func dataSourceWorkRequestFromAny(workRequest any) (generativeaiagentsdk.WorkRequest, error) {
	switch current := workRequest.(type) {
	case generativeaiagentsdk.WorkRequest:
		return current, nil
	case *generativeaiagentsdk.WorkRequest:
		if current == nil {
			return generativeaiagentsdk.WorkRequest{}, fmt.Errorf("DataSource work request is nil")
		}
		return *current, nil
	default:
		return generativeaiagentsdk.WorkRequest{}, fmt.Errorf("unexpected DataSource work request type %T", workRequest)
	}
}

func dataSourceWorkRequestPhaseFromOperationType(
	operationType generativeaiagentsdk.OperationTypeEnum,
) (shared.OSOKAsyncPhase, bool) {
	switch operationType {
	case generativeaiagentsdk.OperationTypeCreateDataSource:
		return shared.OSOKAsyncPhaseCreate, true
	case generativeaiagentsdk.OperationTypeUpdateDataSource:
		return shared.OSOKAsyncPhaseUpdate, true
	case generativeaiagentsdk.OperationTypeDeleteDataSource:
		return shared.OSOKAsyncPhaseDelete, true
	default:
		return "", false
	}
}

func dataSourceWorkRequestActionForPhase(
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

func resolveDataSourceIDFromResources(
	resources []generativeaiagentsdk.WorkRequestResource,
	action generativeaiagentsdk.ActionTypeEnum,
	preferDataSourceOnly bool,
) (string, bool) {
	var candidate string
	for _, resource := range resources {
		if action != "" && resource.ActionType != action {
			continue
		}
		if preferDataSourceOnly && !isDataSourceWorkRequestResource(resource) {
			continue
		}
		id := strings.TrimSpace(stringValue(resource.Identifier))
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

func isDataSourceWorkRequestResource(resource generativeaiagentsdk.WorkRequestResource) bool {
	return normalizeDataSourceWorkRequestToken(stringValue(resource.EntityType)) == "datasource"
}

func normalizeDataSourceWorkRequestToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
