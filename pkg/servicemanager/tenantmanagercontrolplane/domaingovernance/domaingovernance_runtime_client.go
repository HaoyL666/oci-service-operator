/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package domaingovernance

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	tenantmanagercontrolplanesdk "github.com/oracle/oci-go-sdk/v65/tenantmanagercontrolplane"
	tenantmanagercontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/tenantmanagercontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const domainGovernanceDeletePendingMessage = "OCI DomainGovernance delete is in progress"

type domainGovernanceOCIClient interface {
	CreateDomainGovernance(context.Context, tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse, error)
	GetDomainGovernance(context.Context, tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error)
	ListDomainGovernances(context.Context, tenantmanagercontrolplanesdk.ListDomainGovernancesRequest) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error)
	UpdateDomainGovernance(context.Context, tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse, error)
	DeleteDomainGovernance(context.Context, tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest) (tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse, error)
}

type domainGovernanceRuntimeClient struct {
	delegate DomainGovernanceServiceClient
}

type domainGovernanceProjectedStatus struct {
	Id                  string                     `json:"id,omitempty"`
	OwnerId             string                     `json:"ownerId,omitempty"`
	DomainId            string                     `json:"domainId,omitempty"`
	LifecycleState      string                     `json:"lifecycleState,omitempty"`
	OnsTopicId          string                     `json:"onsTopicId,omitempty"`
	OnsSubscriptionId   string                     `json:"onsSubscriptionId,omitempty"`
	IsGovernanceEnabled bool                       `json:"isGovernanceEnabled,omitempty"`
	SubscriptionEmail   string                     `json:"subscriptionEmail,omitempty"`
	TimeCreated         string                     `json:"timeCreated,omitempty"`
	TimeUpdated         string                     `json:"timeUpdated,omitempty"`
	FreeformTags        map[string]string          `json:"freeformTags,omitempty"`
	DefinedTags         map[string]shared.MapValue `json:"definedTags,omitempty"`
	SystemTags          map[string]shared.MapValue `json:"systemTags,omitempty"`
}

type domainGovernanceListCall func(
	context.Context,
	tenantmanagercontrolplanesdk.ListDomainGovernancesRequest,
) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error)

func init() {
	registerDomainGovernanceRuntimeHooksMutator(func(manager *DomainGovernanceServiceManager, hooks *DomainGovernanceRuntimeHooks) {
		ociClient, initErr := newDomainGovernanceRuntimeClient(manager)
		applyDomainGovernanceRuntimeHooks(hooks, ociClient, initErr)
	})
}

func newDomainGovernanceRuntimeClient(manager *DomainGovernanceServiceManager) (domainGovernanceOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("DomainGovernance service manager is nil")
	}

	client, err := tenantmanagercontrolplanesdk.NewDomainGovernanceClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return tenantmanagercontrolplanesdk.DomainGovernanceClient{}, err
	}
	return client, nil
}

func applyDomainGovernanceRuntimeHooks(
	hooks *DomainGovernanceRuntimeHooks,
	ociClient domainGovernanceOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}

	if hooks.Semantics != nil {
		if hooks.Semantics.List != nil {
			hooks.Semantics.List.MatchFields = []string{"domainId", "onsTopicId", "onsSubscriptionId"}
		}
		hooks.Semantics.Lifecycle.ProvisioningStates = []string{"CREATING"}
		hooks.Semantics.Lifecycle.UpdatingStates = []string{"UPDATING"}
		hooks.Semantics.Lifecycle.ActiveStates = []string{"ACTIVE", "INACTIVE"}
		hooks.Semantics.Delete.PendingStates = []string{"DELETING"}
		hooks.Semantics.Delete.TerminalStates = []string{"TERMINATED"}
		hooks.Semantics.Mutation.Mutable = []string{"subscriptionEmail", "isGovernanceEnabled", "freeformTags", "definedTags"}
		hooks.Semantics.Mutation.ForceNew = []string{"compartmentId", "domainId", "onsTopicId", "onsSubscriptionId"}
		hooks.Semantics.CreateFollowUp.Strategy = "none"
		hooks.Semantics.UpdateFollowUp.Strategy = "none"
		hooks.Semantics.DeleteFollowUp.Strategy = "confirm-delete"
		hooks.Semantics.AuxiliaryOperations = nil
	}

	hooks.BuildCreateBody = buildDomainGovernanceCreateBody
	hooks.BuildUpdateBody = buildDomainGovernanceUpdateBody
	hooks.ParityHooks.NormalizeDesiredState = normalizeDomainGovernanceDesiredState
	hooks.ParityHooks.ValidateCreateOnlyDrift = validateDomainGovernanceTrackedCreateOnlyDrift
	hooks.StatusHooks.ProjectStatus = projectDomainGovernanceStatus
	hooks.List.Fields = domainGovernanceListFields()
	hooks.Create.Call = func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest,
	) (tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse, error) {
		if err := domainGovernanceRuntimeClientReady(ociClient, initErr); err != nil {
			return tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse{}, err
		}
		return ociClient.CreateDomainGovernance(ctx, request)
	}
	hooks.Get.Call = func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.GetDomainGovernanceRequest,
	) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
		if err := domainGovernanceRuntimeClientReady(ociClient, initErr); err != nil {
			return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{}, err
		}
		return ociClient.GetDomainGovernance(ctx, request)
	}
	hooks.List.Call = wrapDomainGovernanceListCall(func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.ListDomainGovernancesRequest,
	) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
		if err := domainGovernanceRuntimeClientReady(ociClient, initErr); err != nil {
			return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, err
		}
		return ociClient.ListDomainGovernances(ctx, request)
	})
	hooks.Update.Call = func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest,
	) (tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse, error) {
		if err := domainGovernanceRuntimeClientReady(ociClient, initErr); err != nil {
			return tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse{}, err
		}
		return ociClient.UpdateDomainGovernance(ctx, request)
	}
	hooks.Delete.Call = func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest,
	) (tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse, error) {
		if err := domainGovernanceRuntimeClientReady(ociClient, initErr); err != nil {
			return tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse{}, err
		}
		return ociClient.DeleteDomainGovernance(ctx, request)
	}
	hooks.DeleteHooks.ApplyOutcome = applyDomainGovernanceDeleteOutcome
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate DomainGovernanceServiceClient) DomainGovernanceServiceClient {
		return domainGovernanceRuntimeClient{delegate: delegate}
	})
}

func newDomainGovernanceServiceClientWithClients(
	log loggerutil.OSOKLogger,
	ociClient domainGovernanceOCIClient,
) DomainGovernanceServiceClient {
	hooks := newDomainGovernanceRuntimeHooksWithClients(ociClient)
	applyDomainGovernanceRuntimeHooks(&hooks, ociClient, nil)
	delegate := defaultDomainGovernanceServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*tenantmanagercontrolplanev1beta1.DomainGovernance](
			buildDomainGovernanceGeneratedRuntimeConfig(&DomainGovernanceServiceManager{Log: log}, hooks),
		),
	}
	return wrapDomainGovernanceGeneratedClient(hooks, delegate)
}

func newDomainGovernanceRuntimeHooksWithClients(ociClient domainGovernanceOCIClient) DomainGovernanceRuntimeHooks {
	_ = ociClient
	return newDomainGovernanceDefaultRuntimeHooks(tenantmanagercontrolplanesdk.DomainGovernanceClient{})
}

func domainGovernanceListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{
			FieldName:    "CompartmentId",
			RequestName:  "compartmentId",
			Contribution: "query",
			LookupPaths:  []string{"status.ownerId", "spec.compartmentId", "compartmentId"},
		},
		{
			FieldName:    "DomainId",
			RequestName:  "domainId",
			Contribution: "query",
			LookupPaths:  []string{"status.domainId", "spec.domainId", "domainId"},
		},
		{
			FieldName:        "DomainGovernanceId",
			RequestName:      "domainGovernanceId",
			Contribution:     "query",
			PreferResourceID: true,
		},
		{FieldName: "Page", RequestName: "page", Contribution: "query"},
		{FieldName: "Limit", RequestName: "limit", Contribution: "query"},
		{FieldName: "SortBy", RequestName: "sortBy", Contribution: "query"},
		{FieldName: "SortOrder", RequestName: "sortOrder", Contribution: "query"},
	}
}

func wrapDomainGovernanceListCall(call domainGovernanceListCall) domainGovernanceListCall {
	if call == nil {
		return nil
	}

	return func(
		ctx context.Context,
		request tenantmanagercontrolplanesdk.ListDomainGovernancesRequest,
	) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
		return listDomainGovernancePages(ctx, call, request)
	}
}

func listDomainGovernancePages(
	ctx context.Context,
	call domainGovernanceListCall,
	request tenantmanagercontrolplanesdk.ListDomainGovernancesRequest,
) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
	if call == nil {
		return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, fmt.Errorf("DomainGovernance list call is not configured")
	}

	var combined tenantmanagercontrolplanesdk.ListDomainGovernancesResponse
	seenPages := map[string]struct{}{}
	for {
		pageToken := ""
		if request.Page != nil {
			pageToken = strings.TrimSpace(*request.Page)
		}
		if _, seen := seenPages[pageToken]; seen {
			return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, fmt.Errorf("DomainGovernance list pagination repeated page %q", pageToken)
		}
		seenPages[pageToken] = struct{}{}

		response, err := call(ctx, request)
		if err != nil {
			return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, err
		}
		if combined.RawResponse == nil {
			combined.RawResponse = response.RawResponse
		}
		if combined.OpcRequestId == nil {
			combined.OpcRequestId = response.OpcRequestId
		}
		combined.Items = append(combined.Items, response.Items...)

		if response.OpcNextPage == nil || strings.TrimSpace(*response.OpcNextPage) == "" {
			combined.OpcNextPage = nil
			return combined, nil
		}

		nextPage := strings.TrimSpace(*response.OpcNextPage)
		combined.OpcNextPage = common.String(nextPage)
		request.Page = common.String(nextPage)
	}
}

func buildDomainGovernanceCreateBody(
	_ context.Context,
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
	_ string,
) (any, error) {
	if resource == nil {
		return tenantmanagercontrolplanesdk.CreateDomainGovernanceDetails{}, fmt.Errorf("DomainGovernance resource is nil")
	}
	if err := validateDomainGovernanceSpec(resource.Spec); err != nil {
		return tenantmanagercontrolplanesdk.CreateDomainGovernanceDetails{}, err
	}

	details := tenantmanagercontrolplanesdk.CreateDomainGovernanceDetails{
		CompartmentId:     common.String(strings.TrimSpace(resource.Spec.CompartmentId)),
		DomainId:          common.String(strings.TrimSpace(resource.Spec.DomainId)),
		SubscriptionEmail: common.String(strings.TrimSpace(resource.Spec.SubscriptionEmail)),
		OnsTopicId:        common.String(strings.TrimSpace(resource.Spec.OnsTopicId)),
		OnsSubscriptionId: common.String(strings.TrimSpace(resource.Spec.OnsSubscriptionId)),
	}
	if resource.Spec.FreeformTags != nil {
		details.FreeformTags = maps.Clone(resource.Spec.FreeformTags)
	}
	if resource.Spec.DefinedTags != nil {
		details.DefinedTags = domainGovernanceDefinedTagsFromSpec(resource.Spec.DefinedTags)
	}
	return details, nil
}

func buildDomainGovernanceUpdateBody(
	_ context.Context,
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
	_ string,
	currentResponse any,
) (any, bool, error) {
	if resource == nil {
		return tenantmanagercontrolplanesdk.UpdateDomainGovernanceDetails{}, false, fmt.Errorf("DomainGovernance resource is nil")
	}
	if err := validateDomainGovernanceSpec(resource.Spec); err != nil {
		return tenantmanagercontrolplanesdk.UpdateDomainGovernanceDetails{}, false, err
	}
	if err := validateDomainGovernanceTrackedCreateOnlyDrift(resource, currentResponse); err != nil {
		return tenantmanagercontrolplanesdk.UpdateDomainGovernanceDetails{}, false, err
	}

	current, ok := domainGovernanceProjectionFromResponse(currentResponse)
	if !ok {
		return tenantmanagercontrolplanesdk.UpdateDomainGovernanceDetails{}, false, fmt.Errorf("current DomainGovernance response does not expose a readable projected body")
	}

	details := tenantmanagercontrolplanesdk.UpdateDomainGovernanceDetails{}
	updateNeeded := false
	if desiredEmail := strings.TrimSpace(resource.Spec.SubscriptionEmail); desiredEmail != "" && current.SubscriptionEmail != desiredEmail {
		details.SubscriptionEmail = common.String(desiredEmail)
		updateNeeded = true
	}
	if current.IsGovernanceEnabled != resource.Spec.IsGovernanceEnabled {
		details.IsGovernanceEnabled = common.Bool(resource.Spec.IsGovernanceEnabled)
		updateNeeded = true
	}
	if resource.Spec.FreeformTags != nil {
		desired := maps.Clone(resource.Spec.FreeformTags)
		if !reflect.DeepEqual(current.FreeformTags, desired) {
			details.FreeformTags = desired
			updateNeeded = true
		}
	}
	if resource.Spec.DefinedTags != nil {
		desired := cloneDomainGovernanceDefinedTags(resource.Spec.DefinedTags)
		if !reflect.DeepEqual(current.DefinedTags, desired) {
			details.DefinedTags = domainGovernanceDefinedTagsFromSpec(resource.Spec.DefinedTags)
			updateNeeded = true
		}
	}

	return details, updateNeeded, nil
}

func normalizeDomainGovernanceDesiredState(resource *tenantmanagercontrolplanev1beta1.DomainGovernance, _ any) {
	if resource == nil {
		return
	}

	resource.Spec.CompartmentId = strings.TrimSpace(resource.Spec.CompartmentId)
	resource.Spec.DomainId = strings.TrimSpace(resource.Spec.DomainId)
	resource.Spec.SubscriptionEmail = strings.TrimSpace(resource.Spec.SubscriptionEmail)
	resource.Spec.OnsTopicId = strings.TrimSpace(resource.Spec.OnsTopicId)
	resource.Spec.OnsSubscriptionId = strings.TrimSpace(resource.Spec.OnsSubscriptionId)
}

func validateDomainGovernanceSpec(spec tenantmanagercontrolplanev1beta1.DomainGovernanceSpec) error {
	var missing []string
	if strings.TrimSpace(spec.CompartmentId) == "" {
		missing = append(missing, "compartmentId")
	}
	if strings.TrimSpace(spec.DomainId) == "" {
		missing = append(missing, "domainId")
	}
	if strings.TrimSpace(spec.SubscriptionEmail) == "" {
		missing = append(missing, "subscriptionEmail")
	}
	if strings.TrimSpace(spec.OnsTopicId) == "" {
		missing = append(missing, "onsTopicId")
	}
	if strings.TrimSpace(spec.OnsSubscriptionId) == "" {
		missing = append(missing, "onsSubscriptionId")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("DomainGovernance spec is missing required field(s): %s", strings.Join(missing, ", "))
}

func validateDomainGovernanceTrackedCreateOnlyDrift(
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
	_ any,
) error {
	if resource == nil {
		return fmt.Errorf("DomainGovernance resource is nil")
	}
	if currentCompartmentID := strings.TrimSpace(resource.Status.OwnerId); currentCompartmentID != "" &&
		currentCompartmentID != strings.TrimSpace(resource.Spec.CompartmentId) {
		return fmt.Errorf("DomainGovernance formal semantics require replacement when compartmentId changes")
	}
	if currentDomainID := strings.TrimSpace(resource.Status.DomainId); currentDomainID != "" &&
		currentDomainID != strings.TrimSpace(resource.Spec.DomainId) {
		return fmt.Errorf("DomainGovernance formal semantics require replacement when domainId changes")
	}
	if currentTopicID := strings.TrimSpace(resource.Status.OnsTopicId); currentTopicID != "" &&
		currentTopicID != strings.TrimSpace(resource.Spec.OnsTopicId) {
		return fmt.Errorf("DomainGovernance formal semantics require replacement when onsTopicId changes")
	}
	if currentSubscriptionID := strings.TrimSpace(resource.Status.OnsSubscriptionId); currentSubscriptionID != "" &&
		currentSubscriptionID != strings.TrimSpace(resource.Spec.OnsSubscriptionId) {
		return fmt.Errorf("DomainGovernance formal semantics require replacement when onsSubscriptionId changes")
	}
	return nil
}

func domainGovernanceDefinedTagsFromSpec(spec map[string]shared.MapValue) map[string]map[string]interface{} {
	if spec == nil {
		return nil
	}
	return *util.ConvertToOciDefinedTags(&spec)
}

func applyDomainGovernanceDeleteOutcome(
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
	response any,
	stage generatedruntime.DeleteConfirmStage,
) (generatedruntime.DeleteOutcome, error) {
	if stage == generatedruntime.DeleteConfirmStageAlreadyPending && !domainGovernanceDeleteAlreadyPending(resource) {
		return generatedruntime.DeleteOutcome{}, nil
	}
	if stage == generatedruntime.DeleteConfirmStageAfterRequest ||
		stage == generatedruntime.DeleteConfirmStageAlreadyPending {
		markDomainGovernanceTerminating(resource, response)
		return generatedruntime.DeleteOutcome{Handled: true, Deleted: false}, nil
	}
	return generatedruntime.DeleteOutcome{}, nil
}

func domainGovernanceDeleteAlreadyPending(resource *tenantmanagercontrolplanev1beta1.DomainGovernance) bool {
	if resource == nil {
		return false
	}
	current := resource.Status.OsokStatus.Async.Current
	return current != nil &&
		current.Phase == shared.OSOKAsyncPhaseDelete &&
		current.NormalizedClass == shared.OSOKAsyncClassPending
}

func markDomainGovernanceTerminating(resource *tenantmanagercontrolplanev1beta1.DomainGovernance, response any) {
	if resource == nil {
		return
	}

	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Message = domainGovernanceDeletePendingMessage
	status.Reason = string(shared.Terminating)
	status.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseDelete,
		RawStatus:       domainGovernanceLifecycleState(response),
		NormalizedClass: shared.OSOKAsyncClassPending,
		Message:         domainGovernanceDeletePendingMessage,
		UpdatedAt:       &now,
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", domainGovernanceDeletePendingMessage, loggerutil.OSOKLogger{})
}

func projectDomainGovernanceStatus(resource *tenantmanagercontrolplanev1beta1.DomainGovernance, response any) error {
	if resource == nil {
		return fmt.Errorf("DomainGovernance resource is nil")
	}

	projected, ok := domainGovernanceProjectionFromResponse(response)
	if !ok {
		return fmt.Errorf("DomainGovernance response does not expose a readable projected body")
	}

	resource.Status.Id = projected.Id
	resource.Status.OwnerId = projected.OwnerId
	resource.Status.DomainId = projected.DomainId
	resource.Status.LifecycleState = projected.LifecycleState
	resource.Status.OnsTopicId = projected.OnsTopicId
	resource.Status.OnsSubscriptionId = projected.OnsSubscriptionId
	resource.Status.IsGovernanceEnabled = projected.IsGovernanceEnabled
	resource.Status.SubscriptionEmail = projected.SubscriptionEmail
	resource.Status.TimeCreated = projected.TimeCreated
	resource.Status.TimeUpdated = projected.TimeUpdated
	resource.Status.FreeformTags = cloneDomainGovernanceFreeformTags(projected.FreeformTags)
	resource.Status.DefinedTags = cloneDomainGovernanceDefinedTags(projected.DefinedTags)
	resource.Status.SystemTags = cloneDomainGovernanceDefinedTags(projected.SystemTags)
	return nil
}

func domainGovernanceLifecycleState(response any) string {
	if current, ok := domainGovernanceProjectionFromResponse(response); ok {
		return current.LifecycleState
	}
	return ""
}

func domainGovernanceProjectionFromResponse(response any) (domainGovernanceProjectedStatus, bool) {
	switch typed := response.(type) {
	case domainGovernanceProjectedStatus:
		return typed, true
	case *domainGovernanceProjectedStatus:
		if typed != nil {
			return *typed, true
		}
	case tenantmanagercontrolplanesdk.DomainGovernance:
		return domainGovernanceProjectedStatusFromDomainGovernance(typed), true
	case *tenantmanagercontrolplanesdk.DomainGovernance:
		if typed != nil {
			return domainGovernanceProjectedStatusFromDomainGovernance(*typed), true
		}
	case tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse:
		return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
	case *tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse:
		if typed != nil {
			return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
		}
	case tenantmanagercontrolplanesdk.GetDomainGovernanceResponse:
		return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
	case *tenantmanagercontrolplanesdk.GetDomainGovernanceResponse:
		if typed != nil {
			return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
		}
	case tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse:
		return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
	case *tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse:
		if typed != nil {
			return domainGovernanceProjectedStatusFromDomainGovernance(typed.DomainGovernance), true
		}
	case tenantmanagercontrolplanesdk.DomainGovernanceSummary:
		return domainGovernanceProjectedStatusFromSummary(typed), true
	case *tenantmanagercontrolplanesdk.DomainGovernanceSummary:
		if typed != nil {
			return domainGovernanceProjectedStatusFromSummary(*typed), true
		}
	}
	return domainGovernanceProjectedStatus{}, false
}

func domainGovernanceProjectedStatusFromDomainGovernance(
	source tenantmanagercontrolplanesdk.DomainGovernance,
) domainGovernanceProjectedStatus {
	return domainGovernanceProjectedStatus{
		Id:                  strings.TrimSpace(stringPtrValue(source.Id)),
		OwnerId:             strings.TrimSpace(stringPtrValue(source.OwnerId)),
		DomainId:            strings.TrimSpace(stringPtrValue(source.DomainId)),
		LifecycleState:      string(source.LifecycleState),
		OnsTopicId:          strings.TrimSpace(stringPtrValue(source.OnsTopicId)),
		OnsSubscriptionId:   strings.TrimSpace(stringPtrValue(source.OnsSubscriptionId)),
		IsGovernanceEnabled: boolPtrValue(source.IsGovernanceEnabled),
		SubscriptionEmail:   strings.TrimSpace(stringPtrValue(source.SubscriptionEmail)),
		TimeCreated:         sdkTimeString(source.TimeCreated),
		TimeUpdated:         sdkTimeString(source.TimeUpdated),
		FreeformTags:        cloneDomainGovernanceFreeformTags(source.FreeformTags),
		DefinedTags:         domainGovernanceStatusDefinedTags(source.DefinedTags),
		SystemTags:          domainGovernanceStatusDefinedTags(source.SystemTags),
	}
}

func domainGovernanceProjectedStatusFromSummary(
	source tenantmanagercontrolplanesdk.DomainGovernanceSummary,
) domainGovernanceProjectedStatus {
	return domainGovernanceProjectedStatus{
		Id:                  strings.TrimSpace(stringPtrValue(source.Id)),
		OwnerId:             strings.TrimSpace(stringPtrValue(source.OwnerId)),
		DomainId:            strings.TrimSpace(stringPtrValue(source.DomainId)),
		LifecycleState:      string(source.LifecycleState),
		OnsTopicId:          strings.TrimSpace(stringPtrValue(source.OnsTopicId)),
		OnsSubscriptionId:   strings.TrimSpace(stringPtrValue(source.OnsSubscriptionId)),
		IsGovernanceEnabled: boolPtrValue(source.IsGovernanceEnabled),
		SubscriptionEmail:   strings.TrimSpace(stringPtrValue(source.SubscriptionEmail)),
		TimeCreated:         sdkTimeString(source.TimeCreated),
		TimeUpdated:         sdkTimeString(source.TimeUpdated),
		FreeformTags:        cloneDomainGovernanceFreeformTags(source.FreeformTags),
		DefinedTags:         domainGovernanceStatusDefinedTags(source.DefinedTags),
		SystemTags:          domainGovernanceStatusDefinedTags(source.SystemTags),
	}
}

func cloneDomainGovernanceFreeformTags(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	return maps.Clone(source)
}

func cloneDomainGovernanceDefinedTags(source map[string]shared.MapValue) map[string]shared.MapValue {
	if source == nil {
		return nil
	}

	cloned := make(map[string]shared.MapValue, len(source))
	for namespace, values := range source {
		tagValues := make(shared.MapValue, len(values))
		for key, value := range values {
			tagValues[key] = value
		}
		cloned[namespace] = tagValues
	}
	return cloned
}

func domainGovernanceStatusDefinedTags(source map[string]map[string]interface{}) map[string]shared.MapValue {
	if len(source) == 0 {
		return nil
	}

	converted := make(map[string]shared.MapValue, len(source))
	for namespace, values := range source {
		if len(values) == 0 {
			continue
		}
		tagValues := make(shared.MapValue, len(values))
		for key, value := range values {
			tagValues[key] = fmt.Sprint(value)
		}
		converted[namespace] = tagValues
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func domainGovernanceRuntimeClientReady(client domainGovernanceOCIClient, initErr error) error {
	if initErr != nil {
		return fmt.Errorf("initialize DomainGovernance OCI client: %w", initErr)
	}
	if client == nil {
		return fmt.Errorf("DomainGovernance OCI client is not configured")
	}
	return nil
}

func (c domainGovernanceRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
	req ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if c.delegate == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, fmt.Errorf("DomainGovernance generated runtime delegate is not configured")
	}
	return c.delegate.CreateOrUpdate(ctx, resource, req)
}

func (c domainGovernanceRuntimeClient) Delete(
	ctx context.Context,
	resource *tenantmanagercontrolplanev1beta1.DomainGovernance,
) (bool, error) {
	if c.delegate == nil {
		return false, fmt.Errorf("DomainGovernance generated runtime delegate is not configured")
	}
	return c.delegate.Delete(ctx, resource)
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolPtrValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.String()
}
