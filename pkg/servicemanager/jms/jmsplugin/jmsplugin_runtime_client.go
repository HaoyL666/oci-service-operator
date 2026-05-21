/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package jmsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmssdk "github.com/oracle/oci-go-sdk/v65/jms"
	jmsv1beta1 "github.com/oracle/oci-service-operator/api/jms/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

const jmsPluginKind = "JmsPlugin"

type jmsPluginOCIClient interface {
	CreateJmsPlugin(context.Context, jmssdk.CreateJmsPluginRequest) (jmssdk.CreateJmsPluginResponse, error)
	GetJmsPlugin(context.Context, jmssdk.GetJmsPluginRequest) (jmssdk.GetJmsPluginResponse, error)
	ListJmsPlugins(context.Context, jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error)
	UpdateJmsPlugin(context.Context, jmssdk.UpdateJmsPluginRequest) (jmssdk.UpdateJmsPluginResponse, error)
	DeleteJmsPlugin(context.Context, jmssdk.DeleteJmsPluginRequest) (jmssdk.DeleteJmsPluginResponse, error)
}

func init() {
	registerJmsPluginRuntimeHooksMutator(func(_ *JmsPluginServiceManager, hooks *JmsPluginRuntimeHooks) {
		applyJmsPluginRuntimeHooks(hooks)
	})
}

func applyJmsPluginRuntimeHooks(hooks *JmsPluginRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedJmsPluginRuntimeSemantics()
	hooks.Identity.GuardExistingBeforeCreate = guardJmsPluginExistingBeforeCreate
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *jmsv1beta1.JmsPlugin,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildJmsPluginUpdateBody(resource, currentResponse)
	}
	hooks.Create.Fields = jmsPluginCreateFields()
	hooks.Get.Fields = jmsPluginGetFields()
	hooks.List.Fields = jmsPluginListFields()
	wrapJmsPluginListPages(hooks)
	hooks.Update.Fields = jmsPluginUpdateFields()
	hooks.Delete.Fields = jmsPluginDeleteFields()
}

func newJmsPluginServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client jmsPluginOCIClient,
) JmsPluginServiceClient {
	hooks := newJmsPluginRuntimeHooksWithOCIClient(client)
	applyJmsPluginRuntimeHooks(&hooks)
	delegate := defaultJmsPluginServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*jmsv1beta1.JmsPlugin](
			buildJmsPluginGeneratedRuntimeConfig(&JmsPluginServiceManager{Log: log}, hooks),
		),
	}
	return wrapJmsPluginGeneratedClient(hooks, delegate)
}

func newJmsPluginRuntimeHooksWithOCIClient(client jmsPluginOCIClient) JmsPluginRuntimeHooks {
	hooks := newJmsPluginDefaultRuntimeHooks(jmssdk.JavaManagementServiceClient{})
	hooks.Create.Call = func(ctx context.Context, request jmssdk.CreateJmsPluginRequest) (jmssdk.CreateJmsPluginResponse, error) {
		if client == nil {
			return jmssdk.CreateJmsPluginResponse{}, fmt.Errorf("%s OCI client is nil", jmsPluginKind)
		}
		return client.CreateJmsPlugin(ctx, request)
	}
	hooks.Get.Call = func(ctx context.Context, request jmssdk.GetJmsPluginRequest) (jmssdk.GetJmsPluginResponse, error) {
		if client == nil {
			return jmssdk.GetJmsPluginResponse{}, fmt.Errorf("%s OCI client is nil", jmsPluginKind)
		}
		return client.GetJmsPlugin(ctx, request)
	}
	hooks.List.Call = func(ctx context.Context, request jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error) {
		if client == nil {
			return jmssdk.ListJmsPluginsResponse{}, fmt.Errorf("%s OCI client is nil", jmsPluginKind)
		}
		return client.ListJmsPlugins(ctx, request)
	}
	hooks.Update.Call = func(ctx context.Context, request jmssdk.UpdateJmsPluginRequest) (jmssdk.UpdateJmsPluginResponse, error) {
		if client == nil {
			return jmssdk.UpdateJmsPluginResponse{}, fmt.Errorf("%s OCI client is nil", jmsPluginKind)
		}
		return client.UpdateJmsPlugin(ctx, request)
	}
	hooks.Delete.Call = func(ctx context.Context, request jmssdk.DeleteJmsPluginRequest) (jmssdk.DeleteJmsPluginResponse, error) {
		if client == nil {
			return jmssdk.DeleteJmsPluginResponse{}, fmt.Errorf("%s OCI client is nil", jmsPluginKind)
		}
		return client.DeleteJmsPlugin(ctx, request)
	}
	return hooks
}

func reviewedJmsPluginRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService: "jms",
		FormalSlug:    "jmsplugin",
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "none",
			Runtime:              "generatedruntime",
			FormalClassification: "none",
		},
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "retain-until-confirmed-delete",
		Lifecycle: generatedruntime.LifecycleSemantics{
			ActiveStates: []string{"ACTIVE", "INACTIVE"},
		},
		Delete: generatedruntime.DeleteSemantics{
			Policy:         "required",
			TerminalStates: []string{"DELETED"},
		},
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields:        []string{"compartmentId", "agentId"},
		},
		Mutation: generatedruntime.MutationSemantics{
			Mutable:       []string{"definedTags", "fleetId", "freeformTags"},
			ForceNew:      []string{"agentId", "agentType", "compartmentId"},
			ConflictsWith: map[string][]string{},
		},
		Hooks: generatedruntime.HookSet{
			Create: []generatedruntime.Hook{{Helper: "tfresource.CreateResource", EntityType: jmsPluginKind, Action: "CreateJmsPlugin"}},
			Update: []generatedruntime.Hook{{Helper: "tfresource.UpdateResource", EntityType: jmsPluginKind, Action: "UpdateJmsPlugin"}},
			Delete: []generatedruntime.Hook{{Helper: "tfresource.DeleteResource", EntityType: jmsPluginKind, Action: "DeleteJmsPlugin"}},
		},
		CreateFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "read-after-write",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.CreateResource", EntityType: jmsPluginKind, Action: "GetJmsPlugin"}},
		},
		UpdateFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "read-after-write",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.UpdateResource", EntityType: jmsPluginKind, Action: "GetJmsPlugin"}},
		},
		DeleteFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "confirm-delete",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.DeleteResource", EntityType: jmsPluginKind, Action: "GetJmsPlugin"}},
		},
		AuxiliaryOperations: nil,
		Unsupported:         nil,
	}
}

func jmsPluginCreateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CreateJmsPluginDetails", RequestName: "CreateJmsPluginDetails", Contribution: "body"},
	}
}

func jmsPluginGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "JmsPluginId", RequestName: "jmsPluginId", Contribution: "path", PreferResourceID: true},
	}
}

func jmsPluginListFields() []generatedruntime.RequestField {
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
		{FieldName: "Id", RequestName: "id", Contribution: "query", PreferResourceID: true},
	}
}

func jmsPluginUpdateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "JmsPluginId", RequestName: "jmsPluginId", Contribution: "path", PreferResourceID: true},
		{FieldName: "UpdateJmsPluginDetails", RequestName: "UpdateJmsPluginDetails", Contribution: "body"},
	}
}

func jmsPluginDeleteFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "JmsPluginId", RequestName: "jmsPluginId", Contribution: "path", PreferResourceID: true},
	}
}

func wrapJmsPluginListPages(hooks *JmsPluginRuntimeHooks) {
	if hooks == nil || hooks.List.Call == nil {
		return
	}

	call := hooks.List.Call
	hooks.List.Call = func(ctx context.Context, request jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error) {
		return listJmsPluginPages(ctx, call, request)
	}
}

func listJmsPluginPages(
	ctx context.Context,
	call func(context.Context, jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error),
	request jmssdk.ListJmsPluginsRequest,
) (jmssdk.ListJmsPluginsResponse, error) {
	var combined jmssdk.ListJmsPluginsResponse
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

func guardJmsPluginExistingBeforeCreate(
	_ context.Context,
	resource *jmsv1beta1.JmsPlugin,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s resource is nil", jmsPluginKind)
	}
	if strings.TrimSpace(resource.Spec.CompartmentId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s spec.compartmentId is required", jmsPluginKind)
	}
	if strings.TrimSpace(resource.Spec.AgentId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("%s spec.agentId is required", jmsPluginKind)
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func buildJmsPluginUpdateBody(
	resource *jmsv1beta1.JmsPlugin,
	currentResponse any,
) (jmssdk.UpdateJmsPluginDetails, bool, error) {
	if resource == nil {
		return jmssdk.UpdateJmsPluginDetails{}, false, fmt.Errorf("%s resource is nil", jmsPluginKind)
	}

	current, err := jmsPluginFromResponse(currentResponse)
	if err != nil {
		return jmssdk.UpdateJmsPluginDetails{}, false, err
	}

	details := jmssdk.UpdateJmsPluginDetails{}
	updateNeeded := false

	if desired, ok := jmsPluginDesiredFleetIDUpdate(resource.Spec.FleetId, current.FleetId); ok {
		details.FleetId = desired
		updateNeeded = true
	}
	if desired, ok := jmsPluginDesiredFreeformTagsUpdate(resource.Spec.FreeformTags, current.FreeformTags); ok {
		details.FreeformTags = desired
		updateNeeded = true
	}
	if desired, ok := jmsPluginDesiredDefinedTagsUpdate(resource.Spec.DefinedTags, current.DefinedTags); ok {
		details.DefinedTags = desired
		updateNeeded = true
	}

	return details, updateNeeded, nil
}

func jmsPluginFromResponse(currentResponse any) (jmssdk.JmsPlugin, error) {
	switch current := currentResponse.(type) {
	case jmssdk.JmsPlugin:
		return current, nil
	case *jmssdk.JmsPlugin:
		if current == nil {
			return jmssdk.JmsPlugin{}, fmt.Errorf("current %s response is nil", jmsPluginKind)
		}
		return *current, nil
	case jmssdk.JmsPluginSummary:
		return jmssdk.JmsPlugin{
			Id:                 current.Id,
			AgentId:            current.AgentId,
			AgentType:          current.AgentType,
			LifecycleState:     current.LifecycleState,
			AvailabilityStatus: current.AvailabilityStatus,
			TimeRegistered:     current.TimeRegistered,
			FleetId:            current.FleetId,
			CompartmentId:      current.CompartmentId,
			Hostname:           current.Hostname,
			OsFamily:           current.OsFamily,
			OsArchitecture:     current.OsArchitecture,
			OsDistribution:     current.OsDistribution,
			PluginVersion:      current.PluginVersion,
			TimeLastSeen:       current.TimeLastSeen,
			DefinedTags:        current.DefinedTags,
			FreeformTags:       current.FreeformTags,
			SystemTags:         current.SystemTags,
		}, nil
	case *jmssdk.JmsPluginSummary:
		if current == nil {
			return jmssdk.JmsPlugin{}, fmt.Errorf("current %s summary is nil", jmsPluginKind)
		}
		return jmsPluginFromResponse(*current)
	case jmssdk.GetJmsPluginResponse:
		return current.JmsPlugin, nil
	case *jmssdk.GetJmsPluginResponse:
		if current == nil {
			return jmssdk.JmsPlugin{}, fmt.Errorf("current %s response is nil", jmsPluginKind)
		}
		return current.JmsPlugin, nil
	case jmssdk.CreateJmsPluginResponse:
		return current.JmsPlugin, nil
	case *jmssdk.CreateJmsPluginResponse:
		if current == nil {
			return jmssdk.JmsPlugin{}, fmt.Errorf("current %s response is nil", jmsPluginKind)
		}
		return current.JmsPlugin, nil
	case jmssdk.UpdateJmsPluginResponse:
		return current.JmsPlugin, nil
	case *jmssdk.UpdateJmsPluginResponse:
		if current == nil {
			return jmssdk.JmsPlugin{}, fmt.Errorf("current %s response is nil", jmsPluginKind)
		}
		return current.JmsPlugin, nil
	default:
		return jmssdk.JmsPlugin{}, fmt.Errorf("unexpected current %s response type %T", jmsPluginKind, currentResponse)
	}
}

// Empty string and omission are indistinguishable for JmsPlugin.FleetId, so
// only non-empty desired fleet IDs trigger in-place updates.
func jmsPluginDesiredFleetIDUpdate(spec string, current *string) (*string, bool) {
	desired := strings.TrimSpace(spec)
	if desired == "" {
		return nil, false
	}
	if desired == stringValue(current) {
		return nil, false
	}
	return common.String(desired), true
}

func jmsPluginDesiredFreeformTagsUpdate(
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

func jmsPluginDesiredDefinedTagsUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) (map[string]map[string]interface{}, bool) {
	if spec == nil {
		return nil, false
	}

	desired := jmsPluginDefinedTagsFromSpec(spec)
	if len(desired) == 0 && len(current) == 0 {
		return nil, false
	}
	if jmsPluginJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func jmsPluginDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
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

func jmsPluginJSONEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
