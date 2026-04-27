/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/oracle/oci-go-sdk/v65/common"
	generativeaisdk "github.com/oracle/oci-go-sdk/v65/generativeai"
	generativeaiv1beta1 "github.com/oracle/oci-service-operator/api/generativeai/v1beta1"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

func init() {
	registerEndpointRuntimeHooksMutator(func(_ *EndpointServiceManager, hooks *EndpointRuntimeHooks) {
		applyEndpointRuntimeHooks(hooks)
	})
}

func applyEndpointRuntimeHooks(hooks *EndpointRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedEndpointRuntimeSemantics()
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *generativeaiv1beta1.Endpoint,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildEndpointUpdateBody(resource, currentResponse)
	}
}

func reviewedEndpointRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newEndpointRuntimeSemantics()
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
			"contentModerationConfig",
			"definedTags",
			"description",
			"displayName",
			"freeformTags",
			"generativeAiPrivateEndpointId",
			"piiDetectionConfig",
			"promptInjectionConfig",
		},
		ForceNew: []string{"compartmentId", "dedicatedAiClusterId", "modelId"},
	}
	return semantics
}

func buildEndpointUpdateBody(
	resource *generativeaiv1beta1.Endpoint,
	currentResponse any,
) (generativeaisdk.UpdateEndpointDetails, bool, error) {
	if resource == nil {
		return generativeaisdk.UpdateEndpointDetails{}, false, fmt.Errorf("endpoint resource is nil")
	}

	current, err := endpointRuntimeBody(currentResponse)
	if err != nil {
		return generativeaisdk.UpdateEndpointDetails{}, false, err
	}

	details := generativeaisdk.UpdateEndpointDetails{}
	updateNeeded := false

	if resource.Spec.DisplayName != "" {
		if desired, ok := endpointDesiredStringUpdate(resource.Spec.DisplayName, current.DisplayName); ok {
			details.DisplayName = desired
			updateNeeded = true
		}
	}
	if desired, ok := endpointDesiredStringUpdate(resource.Spec.Description, current.Description); ok {
		details.Description = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredStringUpdate(resource.Spec.GenerativeAiPrivateEndpointId, current.GenerativeAiPrivateEndpointId); ok {
		details.GenerativeAiPrivateEndpointId = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredContentModerationConfigUpdate(resource.Spec.ContentModerationConfig, current.ContentModerationConfig); ok {
		details.ContentModerationConfig = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredPromptInjectionConfigUpdate(resource.Spec.PromptInjectionConfig, current.PromptInjectionConfig); ok {
		details.PromptInjectionConfig = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredPiiDetectionConfigUpdate(resource.Spec.PiiDetectionConfig, current.PiiDetectionConfig); ok {
		details.PiiDetectionConfig = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredFreeformTagsUpdate(resource.Spec.FreeformTags, current.FreeformTags); ok {
		details.FreeformTags = desired
		updateNeeded = true
	}
	if desired, ok := endpointDesiredDefinedTagsUpdate(resource.Spec.DefinedTags, current.DefinedTags); ok {
		details.DefinedTags = desired
		updateNeeded = true
	}

	return details, updateNeeded, nil
}

func endpointRuntimeBody(currentResponse any) (generativeaisdk.Endpoint, error) {
	switch current := currentResponse.(type) {
	case generativeaisdk.Endpoint:
		return current, nil
	case *generativeaisdk.Endpoint:
		if current == nil {
			return generativeaisdk.Endpoint{}, fmt.Errorf("current Endpoint response is nil")
		}
		return *current, nil
	case generativeaisdk.EndpointSummary:
		return generativeaisdk.Endpoint{
			Id:                            current.Id,
			ModelId:                       current.ModelId,
			CompartmentId:                 current.CompartmentId,
			DedicatedAiClusterId:          current.DedicatedAiClusterId,
			TimeCreated:                   current.TimeCreated,
			LifecycleState:                current.LifecycleState,
			DisplayName:                   current.DisplayName,
			Description:                   current.Description,
			GenerativeAiPrivateEndpointId: current.GenerativeAiPrivateEndpointId,
			TimeUpdated:                   current.TimeUpdated,
			LifecycleDetails:              current.LifecycleDetails,
			ContentModerationConfig:       current.ContentModerationConfig,
			PromptInjectionConfig:         current.PromptInjectionConfig,
			PiiDetectionConfig:            current.PiiDetectionConfig,
			FreeformTags:                  current.FreeformTags,
			DefinedTags:                   current.DefinedTags,
			SystemTags:                    current.SystemTags,
		}, nil
	case *generativeaisdk.EndpointSummary:
		if current == nil {
			return generativeaisdk.Endpoint{}, fmt.Errorf("current Endpoint response is nil")
		}
		return endpointRuntimeBody(*current)
	case generativeaisdk.CreateEndpointResponse:
		return current.Endpoint, nil
	case *generativeaisdk.CreateEndpointResponse:
		if current == nil {
			return generativeaisdk.Endpoint{}, fmt.Errorf("current Endpoint response is nil")
		}
		return current.Endpoint, nil
	case generativeaisdk.GetEndpointResponse:
		return current.Endpoint, nil
	case *generativeaisdk.GetEndpointResponse:
		if current == nil {
			return generativeaisdk.Endpoint{}, fmt.Errorf("current Endpoint response is nil")
		}
		return current.Endpoint, nil
	case generativeaisdk.UpdateEndpointResponse:
		return current.Endpoint, nil
	case *generativeaisdk.UpdateEndpointResponse:
		if current == nil {
			return generativeaisdk.Endpoint{}, fmt.Errorf("current Endpoint response is nil")
		}
		return current.Endpoint, nil
	default:
		return generativeaisdk.Endpoint{}, fmt.Errorf("unexpected current Endpoint response type %T", currentResponse)
	}
}

func endpointDesiredStringUpdate(spec string, current *string) (*string, bool) {
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

func endpointDesiredContentModerationConfigUpdate(
	spec generativeaiv1beta1.EndpointContentModerationConfig,
	current *generativeaisdk.ContentModerationConfig,
) (*generativeaisdk.ContentModerationConfig, bool) {
	if endpointContentModerationConfigEqual(spec, current) {
		return nil, false
	}
	return &generativeaisdk.ContentModerationConfig{
		IsEnabled: common.Bool(spec.IsEnabled),
		Mode:      generativeaisdk.ContentModerationConfigModeEnum(spec.Mode),
		ModelId:   endpointOptionalString(spec.ModelId),
	}, true
}

func endpointDesiredPromptInjectionConfigUpdate(
	spec generativeaiv1beta1.EndpointPromptInjectionConfig,
	current *generativeaisdk.PromptInjectionConfig,
) (*generativeaisdk.PromptInjectionConfig, bool) {
	if endpointPromptInjectionConfigEqual(spec, current) {
		return nil, false
	}
	return &generativeaisdk.PromptInjectionConfig{
		IsEnabled: common.Bool(spec.IsEnabled),
		Mode:      generativeaisdk.ContentModerationConfigModeEnum(spec.Mode),
		ModelId:   endpointOptionalString(spec.ModelId),
	}, true
}

func endpointDesiredPiiDetectionConfigUpdate(
	spec generativeaiv1beta1.EndpointPiiDetectionConfig,
	current *generativeaisdk.PiiDetectionConfig,
) (*generativeaisdk.PiiDetectionConfig, bool) {
	if endpointPiiDetectionConfigEqual(spec, current) {
		return nil, false
	}
	return &generativeaisdk.PiiDetectionConfig{
		IsEnabled: common.Bool(spec.IsEnabled),
		Mode:      generativeaisdk.ContentModerationConfigModeEnum(spec.Mode),
		ModelId:   endpointOptionalString(spec.ModelId),
	}, true
}

func endpointDesiredFreeformTagsUpdate(
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

func endpointDesiredDefinedTagsUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) (map[string]map[string]interface{}, bool) {
	if spec == nil {
		return nil, false
	}

	desired := endpointDefinedTagsFromSpec(spec)
	if len(desired) == 0 && len(current) == 0 {
		return nil, false
	}
	if endpointJSONEqual(desired, current) {
		return nil, false
	}
	return desired, true
}

func endpointContentModerationConfigEqual(
	spec generativeaiv1beta1.EndpointContentModerationConfig,
	current *generativeaisdk.ContentModerationConfig,
) bool {
	if current == nil {
		return !endpointContentModerationConfigured(spec)
	}
	return spec.IsEnabled == endpointBoolValue(current.IsEnabled) &&
		spec.Mode == string(current.Mode) &&
		spec.ModelId == endpointStringValue(current.ModelId)
}

func endpointPromptInjectionConfigEqual(
	spec generativeaiv1beta1.EndpointPromptInjectionConfig,
	current *generativeaisdk.PromptInjectionConfig,
) bool {
	if current == nil {
		return !endpointPromptInjectionConfigured(spec)
	}
	return spec.IsEnabled == endpointBoolValue(current.IsEnabled) &&
		spec.Mode == string(current.Mode) &&
		spec.ModelId == endpointStringValue(current.ModelId)
}

func endpointPiiDetectionConfigEqual(
	spec generativeaiv1beta1.EndpointPiiDetectionConfig,
	current *generativeaisdk.PiiDetectionConfig,
) bool {
	if current == nil {
		return !endpointPiiDetectionConfigured(spec)
	}
	return spec.IsEnabled == endpointBoolValue(current.IsEnabled) &&
		spec.Mode == string(current.Mode) &&
		spec.ModelId == endpointStringValue(current.ModelId)
}

func endpointContentModerationConfigured(spec generativeaiv1beta1.EndpointContentModerationConfig) bool {
	return spec.IsEnabled || spec.Mode != "" || spec.ModelId != ""
}

func endpointPromptInjectionConfigured(spec generativeaiv1beta1.EndpointPromptInjectionConfig) bool {
	return spec.IsEnabled || spec.Mode != "" || spec.ModelId != ""
}

func endpointPiiDetectionConfigured(spec generativeaiv1beta1.EndpointPiiDetectionConfig) bool {
	return spec.IsEnabled || spec.Mode != "" || spec.ModelId != ""
}

func endpointDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
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

func endpointJSONEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func endpointOptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return common.String(value)
}

func endpointStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func endpointBoolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}
