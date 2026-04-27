/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package sddc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocvpsdk "github.com/oracle/oci-go-sdk/v65/ocvp"
	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"github.com/oracle/oci-service-operator/pkg/util"
)

type sddcIdentity struct {
	compartmentID string
	displayName   string
}

type sanitizedSddcResponse struct {
	Body map[string]any `presentIn:"body"`
}

func init() {
	registerSddcRuntimeHooksMutator(func(_ *SddcServiceManager, hooks *SddcRuntimeHooks) {
		applySddcRuntimeHooks(hooks)
	})
}

func applySddcRuntimeHooks(hooks *SddcRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedSddcRuntimeSemantics()
	listCall := hooks.List.Call
	hooks.Identity.Resolve = func(resource *ocvpv1beta1.Sddc) (any, error) {
		return resolveSddcIdentity(resource), nil
	}
	hooks.Identity.GuardExistingBeforeCreate = guardSddcExistingBeforeCreate
	hooks.Identity.LookupExisting = func(
		ctx context.Context,
		_ *ocvpv1beta1.Sddc,
		identity any,
	) (any, error) {
		return lookupExistingSddc(ctx, listCall, identity.(sddcIdentity))
	}
	hooks.Read.Get = sddcSanitizedReadOperation(hooks.Get)
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *ocvpv1beta1.Sddc,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildSddcUpdateBody(resource, currentResponse)
	}
	hooks.ParityHooks.ValidateCreateOnlyDrift = validateSddcCreateOnlyDrift
}

func reviewedSddcRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newSddcRuntimeSemantics()
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
			"definedTags",
			"displayName",
			"esxiSoftwareVersion",
			"freeformTags",
			"initialConfiguration",
			"sddcByolAllocationDetails",
			"sshAuthorizedKeys",
			"vmwareSoftwareVersion",
		},
		ForceNew: []string{
			"compartmentId",
			"hcxMode",
			"isSingleHostSddc",
		},
		ConflictsWith: map[string][]string{},
	}
	return semantics
}

func guardSddcExistingBeforeCreate(
	_ context.Context,
	resource *ocvpv1beta1.Sddc,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if sddcIdentityResolutionRequiresDisplayName(resource) {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("Sddc spec.displayName is required when no OCI identifier is recorded")
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func sddcSanitizedReadOperation(
	get runtimeOperationHooks[ocvpsdk.GetSddcRequest, ocvpsdk.GetSddcResponse],
) *generatedruntime.Operation {
	return &generatedruntime.Operation{
		NewRequest: func() any { return &ocvpsdk.GetSddcRequest{} },
		Fields:     append([]generatedruntime.RequestField(nil), get.Fields...),
		Call: func(ctx context.Context, request any) (any, error) {
			response, err := get.Call(ctx, *request.(*ocvpsdk.GetSddcRequest))
			if err != nil {
				return nil, err
			}
			return sanitizeSddcResponse(response.Sddc)
		},
	}
}

func lookupExistingSddc(
	ctx context.Context,
	listCall func(context.Context, ocvpsdk.ListSddcsRequest) (ocvpsdk.ListSddcsResponse, error),
	identity sddcIdentity,
) (any, error) {
	if listCall == nil || identity.compartmentID == "" || identity.displayName == "" {
		return nil, nil
	}

	response, err := listCall(ctx, ocvpsdk.ListSddcsRequest{
		CompartmentId: common.String(identity.compartmentID),
		DisplayName:   common.String(identity.displayName),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve existing Sddc via ListSddcs: %w", err)
	}

	matches := make([]sanitizedSddcResponse, 0, 1)
	for _, item := range response.Items {
		if !isReusableSddcSummary(item, identity.compartmentID, identity.displayName) {
			continue
		}
		sanitized, err := sanitizeSddcSummary(item)
		if err != nil {
			return nil, err
		}
		matches = append(matches, sanitized)
	}

	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf(
			"Sddc bind lookup found multiple reusable OCI resources for compartmentId=%q displayName=%q",
			identity.compartmentID,
			identity.displayName,
		)
	}
}

func resolveSddcIdentity(resource *ocvpv1beta1.Sddc) sddcIdentity {
	if resource == nil {
		return sddcIdentity{}
	}
	return sddcIdentity{
		compartmentID: strings.TrimSpace(resource.Spec.CompartmentId),
		displayName:   strings.TrimSpace(resource.Spec.DisplayName),
	}
}

func sddcIdentityResolutionRequiresDisplayName(resource *ocvpv1beta1.Sddc) bool {
	if resource == nil {
		return false
	}
	if strings.TrimSpace(resource.Spec.DisplayName) != "" {
		return false
	}
	return currentSddcID(resource) == ""
}

func currentSddcID(resource *ocvpv1beta1.Sddc) string {
	if resource == nil {
		return ""
	}
	if ocid := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid)); ocid != "" {
		return ocid
	}
	return strings.TrimSpace(resource.Status.Id)
}

func validateSddcCreateOnlyDrift(resource *ocvpv1beta1.Sddc, currentResponse any) error {
	if resource == nil {
		return nil
	}

	currentBody, ok := sddcResponseBodyMap(currentResponse)
	if !ok {
		return nil
	}

	currentInitialConfiguration, ok := currentBody["initialConfiguration"]
	if !ok {
		return nil
	}

	desiredInitialConfiguration, err := sanitizeSddcComparableValue(
		resource.Spec.InitialConfiguration,
		"Sddc spec initialConfiguration",
	)
	if err != nil {
		return err
	}

	if !sddcComparableValuesEqual(desiredInitialConfiguration, currentInitialConfiguration) {
		return fmt.Errorf("Sddc formal semantics require replacement when initialConfiguration changes")
	}
	return nil
}

func buildSddcUpdateBody(
	resource *ocvpv1beta1.Sddc,
	currentResponse any,
) (ocvpsdk.UpdateSddcDetails, bool, error) {
	if resource == nil {
		return ocvpsdk.UpdateSddcDetails{}, false, fmt.Errorf("Sddc resource is nil")
	}

	current, err := sddcFromComparableResponse(currentResponse)
	if err != nil {
		return ocvpsdk.UpdateSddcDetails{}, false, err
	}

	updateDetails := ocvpsdk.UpdateSddcDetails{}
	updateNeeded := false

	if strings.TrimSpace(resource.Spec.DisplayName) != "" &&
		stringPointerValue(current.DisplayName) != resource.Spec.DisplayName {
		updateDetails.DisplayName = common.String(resource.Spec.DisplayName)
		updateNeeded = true
	}
	if strings.TrimSpace(resource.Spec.VmwareSoftwareVersion) != "" &&
		stringPointerValue(current.VmwareSoftwareVersion) != resource.Spec.VmwareSoftwareVersion {
		updateDetails.VmwareSoftwareVersion = common.String(resource.Spec.VmwareSoftwareVersion)
		updateNeeded = true
	}
	if strings.TrimSpace(resource.Spec.EsxiSoftwareVersion) != "" &&
		stringPointerValue(current.EsxiSoftwareVersion) != resource.Spec.EsxiSoftwareVersion {
		updateDetails.EsxiSoftwareVersion = common.String(resource.Spec.EsxiSoftwareVersion)
		updateNeeded = true
	}
	if strings.TrimSpace(resource.Spec.SshAuthorizedKeys) != "" &&
		stringPointerValue(current.SshAuthorizedKeys) != resource.Spec.SshAuthorizedKeys {
		updateDetails.SshAuthorizedKeys = common.String(resource.Spec.SshAuthorizedKeys)
		updateNeeded = true
	}
	if resource.Spec.FreeformTags != nil {
		desiredFreeformTags := cloneStringMap(resource.Spec.FreeformTags)
		if !reflect.DeepEqual(current.FreeformTags, desiredFreeformTags) {
			updateDetails.FreeformTags = desiredFreeformTags
			updateNeeded = true
		}
	}
	if resource.Spec.DefinedTags != nil {
		desiredDefinedTags := *util.ConvertToOciDefinedTags(&resource.Spec.DefinedTags)
		if !reflect.DeepEqual(current.DefinedTags, desiredDefinedTags) {
			updateDetails.DefinedTags = desiredDefinedTags
			updateNeeded = true
		}
	}
	if sddcByolAllocationSpecified(resource.Spec.SddcByolAllocationDetails) {
		desiredSddcByol := sdkSddcByolAllocationDetailsFromSpec(resource.Spec.SddcByolAllocationDetails)
		if !sddcByolAllocationEqual(current.SddcByolAllocationDetails, desiredSddcByol) {
			updateDetails.SddcByolAllocationDetails = desiredSddcByol
			updateNeeded = true
		}
	}

	if !updateNeeded {
		return ocvpsdk.UpdateSddcDetails{}, false, nil
	}
	return updateDetails, true, nil
}

func sddcResponseBodyMap(currentResponse any) (map[string]any, bool) {
	switch current := currentResponse.(type) {
	case sanitizedSddcResponse:
		if current.Body == nil {
			return nil, false
		}
		return current.Body, true
	case *sanitizedSddcResponse:
		if current == nil || current.Body == nil {
			return nil, false
		}
		return current.Body, true
	default:
		return nil, false
	}
}

func sanitizeSddcComparableValue(value any, label string) (any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", label, err)
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}

	sanitized, _ := sanitizeJSONValue(decoded)
	return sanitized, nil
}

func sddcComparableValuesEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func sddcFromComparableResponse(currentResponse any) (ocvpsdk.Sddc, error) {
	currentBody, ok := sddcResponseBodyMap(currentResponse)
	if !ok {
		return ocvpsdk.Sddc{}, fmt.Errorf("current Sddc response does not expose a sanitized body")
	}

	payload, err := json.Marshal(currentBody)
	if err != nil {
		return ocvpsdk.Sddc{}, fmt.Errorf("marshal current Sddc body: %w", err)
	}

	var current ocvpsdk.Sddc
	if err := json.Unmarshal(payload, &current); err != nil {
		return ocvpsdk.Sddc{}, fmt.Errorf("decode current Sddc body: %w", err)
	}
	return current, nil
}

func sddcByolAllocationSpecified(spec ocvpv1beta1.SddcByolAllocationDetails) bool {
	return strings.TrimSpace(spec.LoadBalancerByolAllocationId) != "" || spec.LoadBalancerInstanceCount != 0
}

func sdkSddcByolAllocationDetailsFromSpec(spec ocvpv1beta1.SddcByolAllocationDetails) *ocvpsdk.SddcByolAllocationDetails {
	details := &ocvpsdk.SddcByolAllocationDetails{}
	if spec.LoadBalancerByolAllocationId != "" {
		details.LoadBalancerByolAllocationId = common.String(spec.LoadBalancerByolAllocationId)
	}
	if spec.LoadBalancerInstanceCount != 0 {
		details.LoadBalancerInstanceCount = common.Int(spec.LoadBalancerInstanceCount)
	}
	return details
}

func sddcByolAllocationEqual(current *ocvpsdk.SddcByolAllocationDetails, desired *ocvpsdk.SddcByolAllocationDetails) bool {
	if desired == nil {
		return current == nil
	}
	if current == nil {
		return false
	}
	return stringPointerValue(current.LoadBalancerByolAllocationId) ==
		stringPointerValue(desired.LoadBalancerByolAllocationId) &&
		intPointerValue(current.LoadBalancerInstanceCount) == intPointerValue(desired.LoadBalancerInstanceCount)
}

func intPointerValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func sanitizeSddcResponse(body ocvpsdk.Sddc) (sanitizedSddcResponse, error) {
	return sanitizeSddcPayload(body, "Sddc response body")
}

func sanitizeSddcSummary(body ocvpsdk.SddcSummary) (sanitizedSddcResponse, error) {
	return sanitizeSddcPayload(body, "Sddc summary body")
}

func sanitizeSddcPayload(body any, label string) (sanitizedSddcResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return sanitizedSddcResponse{}, fmt.Errorf("marshal %s: %w", label, err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return sanitizedSddcResponse{}, fmt.Errorf("decode %s: %w", label, err)
	}

	sanitized, ok := sanitizeJSONValue(decoded)
	if !ok {
		return sanitizedSddcResponse{}, nil
	}
	sanitizedMap, _ := sanitized.(map[string]any)
	return sanitizedSddcResponse{Body: sanitizedMap}, nil
}

func sanitizeJSONValue(value any) (any, bool) {
	switch concrete := value.(type) {
	case nil:
		return nil, false
	case map[string]any:
		cleaned := make(map[string]any, len(concrete))
		for key, child := range concrete {
			sanitizedChild, ok := sanitizeJSONValue(child)
			if !ok {
				continue
			}
			cleaned[key] = sanitizedChild
		}
		if len(cleaned) == 0 {
			return nil, false
		}
		return cleaned, true
	case []any:
		cleaned := make([]any, 0, len(concrete))
		for _, child := range concrete {
			sanitizedChild, ok := sanitizeJSONValue(child)
			if !ok {
				continue
			}
			cleaned = append(cleaned, sanitizedChild)
		}
		if len(cleaned) == 0 {
			return nil, false
		}
		return cleaned, true
	default:
		return concrete, true
	}
}

func isReusableSddcSummary(item ocvpsdk.SddcSummary, compartmentID string, displayName string) bool {
	if stringPointerValue(item.CompartmentId) != compartmentID {
		return false
	}
	if stringPointerValue(item.DisplayName) != displayName {
		return false
	}

	switch strings.TrimSpace(string(item.LifecycleState)) {
	case "ACTIVE", "CREATING", "UPDATING":
		return true
	default:
		return false
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
