/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/oracle/oci-go-sdk/v65/common"
	databasemigrationsdk "github.com/oracle/oci-go-sdk/v65/databasemigration"
	databasemigrationv1beta1 "github.com/oracle/oci-service-operator/api/databasemigration/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

var migrationWorkRequestAsyncAdapter = servicemanager.WorkRequestAsyncAdapter{
	PendingStatusTokens: []string{
		string(databasemigrationsdk.OperationStatusAccepted),
		string(databasemigrationsdk.OperationStatusInProgress),
		string(databasemigrationsdk.OperationStatusWaiting),
		string(databasemigrationsdk.OperationStatusCanceling),
	},
	SucceededStatusTokens: []string{string(databasemigrationsdk.OperationStatusSucceeded)},
	FailedStatusTokens:    []string{string(databasemigrationsdk.OperationStatusFailed)},
	CanceledStatusTokens:  []string{string(databasemigrationsdk.OperationStatusCanceled)},
	CreateActionTokens:    []string{string(databasemigrationsdk.OperationTypesCreateMigration)},
	UpdateActionTokens:    []string{string(databasemigrationsdk.OperationTypesUpdateMigration)},
	DeleteActionTokens:    []string{string(databasemigrationsdk.OperationTypesDeleteMigration)},
}

type migrationOCIClient interface {
	CreateMigration(context.Context, databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error)
	GetMigration(context.Context, databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error)
	ListMigrations(context.Context, databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error)
	UpdateMigration(context.Context, databasemigrationsdk.UpdateMigrationRequest) (databasemigrationsdk.UpdateMigrationResponse, error)
	DeleteMigration(context.Context, databasemigrationsdk.DeleteMigrationRequest) (databasemigrationsdk.DeleteMigrationResponse, error)
	GetWorkRequest(context.Context, databasemigrationsdk.GetWorkRequestRequest) (databasemigrationsdk.GetWorkRequestResponse, error)
}

type migrationLookupIdentity struct {
	CompartmentID        string
	DisplayName          string
	ExactMatchFieldsJSON map[string]any
}

var (
	pendingMigrationCreateBodies sync.Map
	pendingMigrationUpdateBodies sync.Map
	migrationRequestBodySequence atomic.Uint64
)

type migrationRequestBodyContextKey struct{}

func init() {
	registerMigrationRuntimeHooksMutator(func(manager *MigrationServiceManager, hooks *MigrationRuntimeHooks) {
		client, initErr := newMigrationSDKClient(manager)
		applyMigrationRuntimeHooks(hooks, client, initErr)
	})
}

func newMigrationSDKClient(manager *MigrationServiceManager) (migrationOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("Migration service manager is nil")
	}

	client, err := databasemigrationsdk.NewDatabaseMigrationClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyMigrationRuntimeHooks(
	hooks *MigrationRuntimeHooks,
	client migrationOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedMigrationRuntimeSemantics()
	hooks.Identity.Resolve = resolveMigrationIdentity
	hooks.Identity.GuardExistingBeforeCreate = guardMigrationExistingBeforeCreate
	hooks.Identity.LookupExisting = func(
		ctx context.Context,
		resource *databasemigrationv1beta1.Migration,
		identity any,
	) (any, error) {
		return lookupExistingMigration(ctx, client, initErr, resource, identity)
	}
	hooks.Create.Fields = migrationCreateFields()
	hooks.Get.Fields = migrationGetFields()
	hooks.List.Fields = migrationListFields()
	hooks.Update.Fields = migrationUpdateFields()
	hooks.Delete.Fields = migrationDeleteFields()
	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *databasemigrationv1beta1.Migration,
		namespace string,
	) (any, error) {
		return buildMigrationCreateBody(ctx, resource, namespace)
	}
	hooks.BuildUpdateBody = func(
		ctx context.Context,
		resource *databasemigrationv1beta1.Migration,
		namespace string,
		currentResponse any,
	) (any, bool, error) {
		return buildMigrationPreparedUpdateBody(ctx, resource, namespace, currentResponse)
	}
	hooks.Async.Adapter = migrationWorkRequestAsyncAdapter
	hooks.Async.GetWorkRequest = func(ctx context.Context, workRequestID string) (any, error) {
		return getMigrationWorkRequest(ctx, client, initErr, workRequestID)
	}
	hooks.Async.ResolveAction = resolveMigrationGeneratedWorkRequestAction
	hooks.Async.ResolvePhase = resolveMigrationGeneratedWorkRequestPhase
	hooks.Async.RecoverResourceID = recoverMigrationIDFromGeneratedWorkRequest
	hooks.Async.Message = migrationGeneratedWorkRequestMessage
	wrapMigrationRequestBodyContext(hooks)
	wrapMigrationRequestBodies(hooks)
}

func newMigrationServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client migrationOCIClient,
) MigrationServiceClient {
	hooks := newMigrationRuntimeHooksWithOCIClient(client)
	applyMigrationRuntimeHooks(&hooks, client, nil)
	delegate := defaultMigrationServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*databasemigrationv1beta1.Migration](
			buildMigrationGeneratedRuntimeConfig(&MigrationServiceManager{Log: log}, hooks),
		),
	}
	return wrapMigrationGeneratedClient(hooks, delegate)
}

func newMigrationRuntimeConfig(
	log loggerutil.OSOKLogger,
	client migrationOCIClient,
) generatedruntime.Config[*databasemigrationv1beta1.Migration] {
	hooks := newMigrationRuntimeHooksWithOCIClient(client)
	applyMigrationRuntimeHooks(&hooks, client, nil)
	return buildMigrationGeneratedRuntimeConfig(&MigrationServiceManager{Log: log}, hooks)
}

func newMigrationRuntimeHooksWithOCIClient(client migrationOCIClient) MigrationRuntimeHooks {
	return MigrationRuntimeHooks{
		Semantics:       reviewedMigrationRuntimeSemantics(),
		Identity:        generatedruntime.IdentityHooks[*databasemigrationv1beta1.Migration]{},
		Read:            generatedruntime.ReadHooks{},
		TrackedRecreate: generatedruntime.TrackedRecreateHooks[*databasemigrationv1beta1.Migration]{},
		StatusHooks:     generatedruntime.StatusHooks[*databasemigrationv1beta1.Migration]{},
		ParityHooks:     generatedruntime.ParityHooks[*databasemigrationv1beta1.Migration]{},
		Async:           generatedruntime.AsyncHooks[*databasemigrationv1beta1.Migration]{},
		DeleteHooks:     generatedruntime.DeleteHooks[*databasemigrationv1beta1.Migration]{},
		Create: runtimeOperationHooks[databasemigrationsdk.CreateMigrationRequest, databasemigrationsdk.CreateMigrationResponse]{
			Fields: migrationCreateFields(),
			Call: func(ctx context.Context, request databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error) {
				if client == nil {
					return databasemigrationsdk.CreateMigrationResponse{}, fmt.Errorf("Migration OCI client is not configured")
				}
				return client.CreateMigration(ctx, request)
			},
		},
		Get: runtimeOperationHooks[databasemigrationsdk.GetMigrationRequest, databasemigrationsdk.GetMigrationResponse]{
			Fields: migrationGetFields(),
			Call: func(ctx context.Context, request databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error) {
				if client == nil {
					return databasemigrationsdk.GetMigrationResponse{}, fmt.Errorf("Migration OCI client is not configured")
				}
				return client.GetMigration(ctx, request)
			},
		},
		List: runtimeOperationHooks[databasemigrationsdk.ListMigrationsRequest, databasemigrationsdk.ListMigrationsResponse]{
			Fields: migrationListFields(),
			Call: func(ctx context.Context, request databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error) {
				if client == nil {
					return databasemigrationsdk.ListMigrationsResponse{}, fmt.Errorf("Migration OCI client is not configured")
				}
				return client.ListMigrations(ctx, request)
			},
		},
		Update: runtimeOperationHooks[databasemigrationsdk.UpdateMigrationRequest, databasemigrationsdk.UpdateMigrationResponse]{
			Fields: migrationUpdateFields(),
			Call: func(ctx context.Context, request databasemigrationsdk.UpdateMigrationRequest) (databasemigrationsdk.UpdateMigrationResponse, error) {
				if client == nil {
					return databasemigrationsdk.UpdateMigrationResponse{}, fmt.Errorf("Migration OCI client is not configured")
				}
				return client.UpdateMigration(ctx, request)
			},
		},
		Delete: runtimeOperationHooks[databasemigrationsdk.DeleteMigrationRequest, databasemigrationsdk.DeleteMigrationResponse]{
			Fields: migrationDeleteFields(),
			Call: func(ctx context.Context, request databasemigrationsdk.DeleteMigrationRequest) (databasemigrationsdk.DeleteMigrationResponse, error) {
				if client == nil {
					return databasemigrationsdk.DeleteMigrationResponse{}, fmt.Errorf("Migration OCI client is not configured")
				}
				return client.DeleteMigration(ctx, request)
			},
		},
		WrapGeneratedClient: []func(MigrationServiceClient) MigrationServiceClient{},
	}
}

func reviewedMigrationRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService: "databasemigration",
		FormalSlug:    "migration",
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "workrequest",
			Runtime:              "generatedruntime",
			FormalClassification: "workrequest",
			WorkRequest: &generatedruntime.WorkRequestSemantics{
				Source: "service-sdk",
				Phases: []string{"create", "update", "delete"},
			},
		},
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "retain-until-confirmed-delete",
		Lifecycle: generatedruntime.LifecycleSemantics{
			ProvisioningStates: []string{
				string(databasemigrationsdk.MigrationLifecycleStatesAccepted),
				string(databasemigrationsdk.MigrationLifecycleStatesCreating),
				string(databasemigrationsdk.MigrationLifecycleStatesInProgress),
				string(databasemigrationsdk.MigrationLifecycleStatesWaiting),
			},
			UpdatingStates: []string{string(databasemigrationsdk.MigrationLifecycleStatesUpdating)},
			ActiveStates: []string{
				string(databasemigrationsdk.MigrationLifecycleStatesActive),
				string(databasemigrationsdk.MigrationLifecycleStatesInactive),
				string(databasemigrationsdk.MigrationLifecycleStatesSucceeded),
			},
		},
		Delete: generatedruntime.DeleteSemantics{
			Policy:         "required",
			PendingStates:  []string{string(databasemigrationsdk.MigrationLifecycleStatesDeleting)},
			TerminalStates: []string{string(databasemigrationsdk.MigrationLifecycleStatesDeleted)},
		},
		List: nil,
		Mutation: generatedruntime.MutationSemantics{
			Mutable: []string{
				"advisorSettings",
				"advancedParameters",
				"dataTransferMediumDetails",
				"definedTags",
				"description",
				"displayName",
				"freeformTags",
				"ggsDetails",
				"hubDetails",
				"initialLoadSettings",
				"sourceContainerDatabaseConnectionId",
				"sourceDatabaseConnectionId",
				"sourceStandbyDatabaseConnectionId",
				"targetDatabaseConnectionId",
				"type",
			},
			ForceNew:      []string{"assessmentId", "compartmentId", "databaseCombination"},
			ConflictsWith: map[string][]string{},
		},
		Hooks: generatedruntime.HookSet{
			Create: []generatedruntime.Hook{{Helper: "tfresource.CreateResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "CREATED"}},
			Update: []generatedruntime.Hook{{Helper: "tfresource.UpdateResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "UPDATED"}},
			Delete: []generatedruntime.Hook{{Helper: "tfresource.DeleteResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "DELETED"}},
		},
		CreateFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "GetWorkRequest -> GetMigration",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.CreateResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "CREATED"}},
		},
		UpdateFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "GetWorkRequest -> GetMigration",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.UpdateResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "UPDATED"}},
		},
		DeleteFollowUp: generatedruntime.FollowUpSemantics{
			Strategy: "GetWorkRequest -> GetMigration/ListMigrations confirm-delete",
			Hooks:    []generatedruntime.Hook{{Helper: "tfresource.DeleteResource"}, {Helper: "tfresource.WaitForWorkRequestWithErrorHandling", EntityType: "migration", Action: "DELETED"}},
		},
		AuxiliaryOperations: []generatedruntime.AuxiliaryOperation{},
		Unsupported:         []generatedruntime.UnsupportedSemantic{},
	}
}

func migrationCreateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CreateMigrationDetails", RequestName: "CreateMigrationDetails", Contribution: "body"},
	}
}

func migrationGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MigrationId", RequestName: "migrationId", Contribution: "path", PreferResourceID: true},
	}
}

func migrationListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CompartmentId", RequestName: "compartmentId", Contribution: "query"},
		{FieldName: "DisplayName", RequestName: "displayName", Contribution: "query"},
		{FieldName: "LifecycleState", RequestName: "lifecycleState", Contribution: "query"},
		{FieldName: "LifecycleDetails", RequestName: "lifecycleDetails", Contribution: "query"},
	}
}

func migrationUpdateFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MigrationId", RequestName: "migrationId", Contribution: "path", PreferResourceID: true},
		{FieldName: "UpdateMigrationDetails", RequestName: "UpdateMigrationDetails", Contribution: "body"},
	}
}

func migrationDeleteFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "MigrationId", RequestName: "migrationId", Contribution: "path", PreferResourceID: true},
	}
}

func resolveMigrationIdentity(resource *databasemigrationv1beta1.Migration) (any, error) {
	if resource == nil {
		return nil, nil
	}

	identity := migrationLookupIdentity{
		CompartmentID:        strings.TrimSpace(resource.Spec.CompartmentId),
		DisplayName:          strings.TrimSpace(resource.Spec.DisplayName),
		ExactMatchFieldsJSON: map[string]any{},
	}
	if identity.CompartmentID != "" {
		identity.ExactMatchFieldsJSON["compartmentId"] = identity.CompartmentID
	}
	if identity.DisplayName != "" {
		identity.ExactMatchFieldsJSON["displayName"] = identity.DisplayName
	}
	if combination, ok := normalizeMigrationDatabaseCombination(resource.Spec.DatabaseCombination); ok {
		identity.ExactMatchFieldsJSON["databaseCombination"] = combination
	}
	if migrationType, ok := normalizeMigrationType(resource.Spec.Type); ok {
		identity.ExactMatchFieldsJSON["type"] = migrationType
	}
	if sourceID := strings.TrimSpace(resource.Spec.SourceDatabaseConnectionId); sourceID != "" {
		identity.ExactMatchFieldsJSON["sourceDatabaseConnectionId"] = sourceID
	}
	if targetID := strings.TrimSpace(resource.Spec.TargetDatabaseConnectionId); targetID != "" {
		identity.ExactMatchFieldsJSON["targetDatabaseConnectionId"] = targetID
	}
	if assessmentID := strings.TrimSpace(resource.Spec.AssessmentId); assessmentID != "" {
		identity.ExactMatchFieldsJSON["assessmentId"] = assessmentID
	}
	return identity, nil
}

func guardMigrationExistingBeforeCreate(
	_ context.Context,
	resource *databasemigrationv1beta1.Migration,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if resource == nil {
		return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
	}
	if strings.TrimSpace(resource.Spec.CompartmentId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	if strings.TrimSpace(resource.Spec.DisplayName) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	if _, ok := normalizeMigrationDatabaseCombination(resource.Spec.DatabaseCombination); !ok {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	if _, ok := normalizeMigrationType(resource.Spec.Type); !ok {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	if strings.TrimSpace(resource.Spec.SourceDatabaseConnectionId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	if strings.TrimSpace(resource.Spec.TargetDatabaseConnectionId) == "" {
		return generatedruntime.ExistingBeforeCreateDecisionSkip, nil
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func lookupExistingMigration(
	ctx context.Context,
	client migrationOCIClient,
	initErr error,
	_ *databasemigrationv1beta1.Migration,
	identity any,
) (any, error) {
	if identity == nil {
		return nil, nil
	}
	if initErr != nil {
		return nil, fmt.Errorf("initialize Migration OCI client: %w", initErr)
	}
	if client == nil {
		return nil, fmt.Errorf("Migration OCI client is not configured")
	}

	lookup, ok := identity.(migrationLookupIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected Migration lookup identity type %T", identity)
	}
	if lookup.CompartmentID == "" || lookup.DisplayName == "" {
		return nil, nil
	}

	response, err := client.ListMigrations(ctx, databasemigrationsdk.ListMigrationsRequest{
		CompartmentId: common.String(lookup.CompartmentID),
		DisplayName:   common.String(lookup.DisplayName),
	})
	if err != nil {
		return nil, err
	}

	for _, summary := range response.Items {
		if !migrationMatchesLookup(summary, lookup.ExactMatchFieldsJSON) {
			continue
		}

		getResponse, err := client.GetMigration(ctx, databasemigrationsdk.GetMigrationRequest{
			MigrationId: summary.GetId(),
		})
		if err != nil {
			return nil, err
		}
		if migrationMatchesLookup(getResponse.Migration, lookup.ExactMatchFieldsJSON) {
			return getResponse, nil
		}
	}

	return nil, nil
}

func migrationMatchesLookup(payload any, exactMatchFields map[string]any) bool {
	if len(exactMatchFields) == 0 {
		return false
	}
	values, err := migrationJSONMap(payload)
	if err != nil {
		return false
	}
	return migrationMapSubsetEqual(exactMatchFields, values)
}

func buildMigrationCreateDetails(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	namespace string,
) (databasemigrationsdk.CreateMigrationDetails, error) {
	if resource == nil {
		return nil, fmt.Errorf("Migration resource is nil")
	}
	if err := validateMigrationRequiredFields(resource); err != nil {
		return nil, err
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValue(resource, ctx, nil, namespace)
	if err != nil {
		return nil, err
	}

	combination, err := migrationDatabaseCombinationForResource(resource, nil)
	if err != nil {
		return nil, err
	}

	specValues, err := migrationJSONMap(resolvedSpec)
	if err != nil {
		return nil, fmt.Errorf("project Migration create spec: %w", err)
	}
	if err := validateMigrationUnsupportedSpec(combination, specValues); err != nil {
		return nil, err
	}

	return decodeMigrationCreateDetails(combination, resolvedSpec)
}

func buildMigrationCreateBody(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	namespace string,
) (any, error) {
	details, err := buildMigrationCreateDetails(ctx, resource, namespace)
	if err != nil {
		return nil, err
	}
	if err := stashMigrationCreateBody(ctx, resource, details); err != nil {
		return nil, err
	}
	return nil, nil
}

func buildMigrationUpdateDetails(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	namespace string,
	currentResponse any,
) (databasemigrationsdk.UpdateMigrationDetails, bool, error) {
	if resource == nil {
		return nil, false, fmt.Errorf("Migration resource is nil")
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValue(resource, ctx, nil, namespace)
	if err != nil {
		return nil, false, err
	}

	combination, err := migrationDatabaseCombinationForResource(resource, currentResponse)
	if err != nil {
		return nil, false, err
	}

	specValues, err := migrationJSONMap(resolvedSpec)
	if err != nil {
		return nil, false, fmt.Errorf("project Migration update spec: %w", err)
	}
	if err := validateMigrationUnsupportedSpec(combination, specValues); err != nil {
		return nil, false, err
	}

	desired, err := decodeMigrationUpdateDetails(combination, resolvedSpec)
	if err != nil {
		return nil, false, err
	}
	desiredValues, err := migrationJSONMap(desired)
	if err != nil {
		return nil, false, fmt.Errorf("project desired Migration update body: %w", err)
	}
	migrationPruneComparableUpdateFields(desiredValues)
	if len(desiredValues) == 0 {
		return desired, false, nil
	}

	currentDetails, err := migrationCurrentUpdateDetails(combination, currentResponse)
	if err != nil {
		return nil, false, err
	}
	currentValues, err := migrationJSONMap(currentDetails)
	if err != nil {
		return nil, false, fmt.Errorf("project current Migration update body: %w", err)
	}
	migrationPruneComparableUpdateFields(currentValues)

	return desired, !migrationMapSubsetEqual(desiredValues, currentValues), nil
}

func buildMigrationUpdateBody(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	namespace string,
	currentResponse any,
) (any, bool, error) {
	details, updateNeeded, err := buildMigrationUpdateDetails(ctx, resource, namespace, currentResponse)
	if err != nil {
		return nil, false, err
	}
	if !updateNeeded {
		return nil, false, nil
	}
	return details, true, nil
}

func buildMigrationPreparedUpdateBody(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	namespace string,
	currentResponse any,
) (any, bool, error) {
	details, updateNeeded, err := buildMigrationUpdateDetails(ctx, resource, namespace, currentResponse)
	if err != nil {
		return nil, false, err
	}
	if !updateNeeded {
		return nil, false, nil
	}
	if err := stashMigrationUpdateBody(ctx, resource, details); err != nil {
		return nil, false, err
	}
	return nil, true, nil
}

func migrationCurrentUpdateDetails(
	databaseCombination string,
	currentResponse any,
) (databasemigrationsdk.UpdateMigrationDetails, error) {
	body, err := migrationRuntimeBody(currentResponse)
	if err != nil {
		return nil, err
	}
	return decodeMigrationUpdateDetails(databaseCombination, body)
}

func validateMigrationRequiredFields(resource *databasemigrationv1beta1.Migration) error {
	var missing []string
	if strings.TrimSpace(resource.Spec.CompartmentId) == "" {
		missing = append(missing, "spec.compartmentId")
	}
	if strings.TrimSpace(resource.Spec.DatabaseCombination) == "" {
		missing = append(missing, "spec.databaseCombination")
	}
	if strings.TrimSpace(resource.Spec.Type) == "" {
		missing = append(missing, "spec.type")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("Migration requires %s", strings.Join(missing, ", "))
}

func validateMigrationUnsupportedSpec(
	databaseCombination string,
	specValues map[string]any,
) error {
	var problems []string

	var helperFields []string
	for _, path := range []string{"excludeObjects", "includeObjects", "bulkIncludeExcludeData"} {
		if migrationHasMeaningfulValue(specValues, path) {
			helperFields = append(helperFields, "spec."+path)
		}
	}
	if len(helperFields) > 0 {
		problems = append(problems, fmt.Sprintf(
			"Migration object helper fields are out of scope for the published runtime: %s",
			strings.Join(helperFields, ", "),
		))
	}

	switch databaseCombination {
	case "MYSQL":
		var unsupported []string
		for _, path := range []string{
			"advancedParameters",
			"sourceContainerDatabaseConnectionId",
			"sourceStandbyDatabaseConnectionId",
		} {
			if migrationHasMeaningfulValue(specValues, path) {
				unsupported = append(unsupported, "spec."+path)
			}
		}
		if len(unsupported) > 0 {
			problems = append(problems, fmt.Sprintf(
				"Migration databaseCombination MYSQL does not support %s",
				strings.Join(unsupported, ", "),
			))
		}
	case "ORACLE":
		var unsupported []string
		for _, path := range []string{
			"initialLoadSettings.compatibility",
			"initialLoadSettings.handleGrantErrors",
			"initialLoadSettings.isConsistent",
			"initialLoadSettings.isIgnoreExistingObjects",
			"initialLoadSettings.isTzUtc",
			"initialLoadSettings.primaryKeyCompatibility",
		} {
			if migrationHasMeaningfulValue(specValues, path) {
				unsupported = append(unsupported, "spec."+path)
			}
		}
		if len(unsupported) > 0 {
			problems = append(problems, fmt.Sprintf(
				"Migration databaseCombination ORACLE does not support %s",
				strings.Join(unsupported, ", "),
			))
		}
	}

	if migrationHasMeaningfulValue(specValues, "dataTransferMediumDetails") {
		rawType, _ := migrationValueByPath(specValues, "dataTransferMediumDetails.type")
		mediumType := strings.ToUpper(strings.TrimSpace(fmt.Sprint(rawType)))
		switch mediumType {
		case "":
			problems = append(problems, "Migration published runtime requires spec.dataTransferMediumDetails.type when dataTransferMediumDetails is set")
		case "OBJECT_STORAGE":
		default:
			problems = append(problems, "Migration published runtime only supports spec.dataTransferMediumDetails.type=OBJECT_STORAGE")
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}

func migrationDatabaseCombinationForResource(
	resource *databasemigrationv1beta1.Migration,
	currentResponse any,
) (string, error) {
	if resource != nil {
		if combination, ok := normalizeMigrationDatabaseCombination(resource.Spec.DatabaseCombination); ok {
			return combination, nil
		}
	}
	if currentResponse != nil {
		body, err := migrationRuntimeBody(currentResponse)
		if err != nil {
			return "", err
		}
		if combination, ok := normalizeMigrationDatabaseCombination(migrationDatabaseCombinationFromRuntimeBody(body)); ok {
			return combination, nil
		}
	}
	return "", fmt.Errorf("Migration spec.databaseCombination must be set to MYSQL or ORACLE")
}

func normalizeMigrationDatabaseCombination(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MYSQL":
		return "MYSQL", true
	case "ORACLE":
		return "ORACLE", true
	default:
		return "", false
	}
}

func normalizeMigrationType(raw string) (string, bool) {
	migrationType, ok := databasemigrationsdk.GetMappingMigrationTypesEnum(strings.TrimSpace(raw))
	if !ok {
		return "", false
	}
	return string(migrationType), true
}

func decodeMigrationCreateDetails(
	databaseCombination string,
	raw any,
) (databasemigrationsdk.CreateMigrationDetails, error) {
	switch databaseCombination {
	case "MYSQL":
		details, err := decodeMigrationConcrete[databasemigrationsdk.CreateMySqlMigrationDetails](raw)
		return details, err
	case "ORACLE":
		details, err := decodeMigrationConcrete[databasemigrationsdk.CreateOracleMigrationDetails](raw)
		return details, err
	default:
		return nil, fmt.Errorf("unsupported Migration databaseCombination %q", databaseCombination)
	}
}

func decodeMigrationUpdateDetails(
	databaseCombination string,
	raw any,
) (databasemigrationsdk.UpdateMigrationDetails, error) {
	switch databaseCombination {
	case "MYSQL":
		details, err := decodeMigrationConcrete[databasemigrationsdk.UpdateMySqlMigrationDetails](raw)
		return details, err
	case "ORACLE":
		details, err := decodeMigrationConcrete[databasemigrationsdk.UpdateOracleMigrationDetails](raw)
		return details, err
	default:
		return nil, fmt.Errorf("unsupported Migration databaseCombination %q", databaseCombination)
	}
}

func decodeMigrationConcrete[T any](raw any) (T, error) {
	var decoded T

	payload, err := json.Marshal(raw)
	if err != nil {
		return decoded, fmt.Errorf("marshal Migration payload: %w", err)
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return decoded, fmt.Errorf("unmarshal Migration payload: %w", err)
	}
	return decoded, nil
}

func migrationRuntimeBody(currentResponse any) (any, error) {
	switch current := currentResponse.(type) {
	case databasemigrationsdk.CreateMigrationResponse:
		return current.Migration, nil
	case *databasemigrationsdk.CreateMigrationResponse:
		if current == nil {
			return nil, fmt.Errorf("current Migration response is nil")
		}
		return current.Migration, nil
	case databasemigrationsdk.GetMigrationResponse:
		return current.Migration, nil
	case *databasemigrationsdk.GetMigrationResponse:
		if current == nil {
			return nil, fmt.Errorf("current Migration response is nil")
		}
		return current.Migration, nil
	case databasemigrationsdk.Migration:
		if current == nil {
			return nil, fmt.Errorf("current Migration body is nil")
		}
		return current, nil
	case databasemigrationsdk.MigrationSummary:
		if current == nil {
			return nil, fmt.Errorf("current Migration summary is nil")
		}
		return current, nil
	case databasemigrationsdk.MySqlMigration,
		databasemigrationsdk.MySqlMigrationSummary,
		databasemigrationsdk.OracleMigration,
		databasemigrationsdk.OracleMigrationSummary:
		return current, nil
	default:
		return nil, fmt.Errorf("unsupported current Migration payload type %T", currentResponse)
	}
}

func migrationDatabaseCombinationFromRuntimeBody(body any) string {
	switch body.(type) {
	case databasemigrationsdk.MySqlMigration, databasemigrationsdk.MySqlMigrationSummary:
		return "MYSQL"
	case databasemigrationsdk.OracleMigration, databasemigrationsdk.OracleMigrationSummary:
		return "ORACLE"
	default:
		values, err := migrationJSONMap(body)
		if err != nil {
			return ""
		}
		raw, _ := values["databaseCombination"].(string)
		return raw
	}
}

func migrationHasMeaningfulValue(values map[string]any, path string) bool {
	value, ok := migrationValueByPath(values, path)
	return ok && migrationValueIsMeaningful(value)
}

func migrationValueByPath(values map[string]any, path string) (any, bool) {
	if len(values) == 0 {
		return nil, false
	}

	current := any(values)
	for _, segment := range strings.Split(path, ".") {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = asMap[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func migrationValueIsMeaningful(value any) bool {
	switch concrete := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(concrete) != ""
	case bool:
		return concrete
	case float64:
		return concrete != 0
	case []any:
		return len(concrete) > 0
	case map[string]any:
		for _, child := range concrete {
			if migrationValueIsMeaningful(child) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func migrationJSONMap(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func migrationMapSubsetEqual(desired map[string]any, current map[string]any) bool {
	for key, desiredValue := range desired {
		currentValue, ok := current[key]
		if !ok || !migrationValuesEqual(desiredValue, currentValue) {
			return false
		}
	}
	return true
}

func migrationValuesEqual(left any, right any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return fmt.Sprint(left) == fmt.Sprint(right)
	}
	return string(leftPayload) == string(rightPayload)
}

func migrationPruneComparableUpdateFields(values map[string]any) {
	if len(values) == 0 {
		return
	}
	migrationDeleteMapPath(values, strings.Split("hubDetails.restAdminCredentials.password", "."))
}

func migrationDeleteMapPath(values map[string]any, segments []string) bool {
	if len(segments) == 0 || len(values) == 0 {
		return len(values) == 0
	}

	segment := segments[0]
	if len(segments) == 1 {
		delete(values, segment)
		return len(values) == 0
	}

	child, ok := values[segment].(map[string]any)
	if !ok {
		return false
	}
	if migrationDeleteMapPath(child, segments[1:]) {
		delete(values, segment)
	}
	return len(values) == 0
}

func wrapMigrationRequestBodyContext(hooks *MigrationRuntimeHooks) {
	if hooks == nil {
		return
	}
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate MigrationServiceClient) MigrationServiceClient {
		return migrationRequestBodyContextClient{delegate: delegate}
	})
}

type migrationRequestBodyContextClient struct {
	delegate MigrationServiceClient
}

func (c migrationRequestBodyContextClient) CreateOrUpdate(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	req ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	ctx = withMigrationRequestBodyToken(ctx)
	defer clearMigrationRequestBodies(ctx)
	return c.delegate.CreateOrUpdate(ctx, resource, req)
}

func (c migrationRequestBodyContextClient) Delete(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
) (bool, error) {
	return c.delegate.Delete(ctx, resource)
}

func withMigrationRequestBodyToken(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if migrationRequestBodyToken(ctx) != "" {
		return ctx
	}
	token := fmt.Sprintf("migration-request-%d", migrationRequestBodySequence.Add(1))
	return context.WithValue(ctx, migrationRequestBodyContextKey{}, token)
}

func migrationRequestBodyToken(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(migrationRequestBodyContextKey{}).(string)
	return strings.TrimSpace(value)
}

func clearMigrationRequestBodies(ctx context.Context) {
	key := migrationRequestBodyToken(ctx)
	if key == "" {
		return
	}
	pendingMigrationCreateBodies.Delete(key)
	pendingMigrationUpdateBodies.Delete(key)
}

func wrapMigrationRequestBodies(hooks *MigrationRuntimeHooks) {
	if hooks == nil {
		return
	}

	createCall := hooks.Create.Call
	if createCall != nil {
		hooks.Create.Call = func(ctx context.Context, request databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error) {
			if request.CreateMigrationDetails == nil {
				body, err := takeMigrationCreateBody(ctx, request.OpcRetryToken)
				if err != nil {
					return databasemigrationsdk.CreateMigrationResponse{}, err
				}
				request.CreateMigrationDetails = body
			}
			return createCall(ctx, request)
		}
	}

	updateCall := hooks.Update.Call
	if updateCall != nil {
		hooks.Update.Call = func(ctx context.Context, request databasemigrationsdk.UpdateMigrationRequest) (databasemigrationsdk.UpdateMigrationResponse, error) {
			if request.UpdateMigrationDetails == nil {
				body, err := takeMigrationUpdateBody(ctx, request.MigrationId)
				if err != nil {
					return databasemigrationsdk.UpdateMigrationResponse{}, err
				}
				request.UpdateMigrationDetails = body
			}
			return updateCall(ctx, request)
		}
	}
}

func stashMigrationCreateBody(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	body databasemigrationsdk.CreateMigrationDetails,
) error {
	key := migrationRequestBodyToken(ctx)
	if key == "" {
		key = migrationResourceBodyKey(resource)
	}
	if key == "" {
		return fmt.Errorf("Migration create body cannot be keyed without resource namespace/name or uid")
	}
	pendingMigrationCreateBodies.Store(key, body)
	return nil
}

func takeMigrationCreateBody(
	ctx context.Context,
	retryToken *string,
) (databasemigrationsdk.CreateMigrationDetails, error) {
	key := migrationRequestBodyToken(ctx)
	if key == "" {
		key = migrationStringValue(retryToken)
	}
	if key == "" {
		return nil, fmt.Errorf("Migration create body is missing a request key")
	}
	value, ok := pendingMigrationCreateBodies.LoadAndDelete(key)
	if !ok {
		return nil, fmt.Errorf("Migration create body was not prepared for key %q", key)
	}
	body, ok := value.(databasemigrationsdk.CreateMigrationDetails)
	if !ok {
		return nil, fmt.Errorf("prepared Migration create body has unexpected type %T", value)
	}
	return body, nil
}

func stashMigrationUpdateBody(
	ctx context.Context,
	resource *databasemigrationv1beta1.Migration,
	body databasemigrationsdk.UpdateMigrationDetails,
) error {
	key := migrationRequestBodyToken(ctx)
	if key == "" {
		key = trackedMigrationID(resource)
	}
	if key == "" {
		return fmt.Errorf("Migration update body cannot be keyed without a tracked Migration OCID")
	}
	pendingMigrationUpdateBodies.Store(key, body)
	return nil
}

func takeMigrationUpdateBody(
	ctx context.Context,
	migrationID *string,
) (databasemigrationsdk.UpdateMigrationDetails, error) {
	key := migrationRequestBodyToken(ctx)
	if key == "" {
		key = migrationStringValue(migrationID)
	}
	if key == "" {
		return nil, fmt.Errorf("Migration update body is missing a resource key")
	}
	value, ok := pendingMigrationUpdateBodies.LoadAndDelete(key)
	if !ok {
		return nil, fmt.Errorf("Migration update body was not prepared for key %q", key)
	}
	body, ok := value.(databasemigrationsdk.UpdateMigrationDetails)
	if !ok {
		return nil, fmt.Errorf("prepared Migration update body has unexpected type %T", value)
	}
	return body, nil
}

func trackedMigrationID(resource *databasemigrationv1beta1.Migration) string {
	if resource == nil {
		return ""
	}
	if id := strings.TrimSpace(resource.Status.Id); id != "" {
		return id
	}
	return strings.TrimSpace(string(resource.Status.OsokStatus.Ocid))
}

func migrationResourceBodyKey(resource *databasemigrationv1beta1.Migration) string {
	if resource == nil {
		return ""
	}
	if uid := strings.TrimSpace(string(resource.UID)); uid != "" {
		return uid
	}
	namespace := strings.TrimSpace(resource.Namespace)
	name := strings.TrimSpace(resource.Name)
	if namespace == "" && name == "" {
		return ""
	}
	return namespace + "/" + name
}

func getMigrationWorkRequest(
	ctx context.Context,
	client migrationOCIClient,
	initErr error,
	workRequestID string,
) (any, error) {
	if initErr != nil {
		return nil, fmt.Errorf("initialize Migration OCI client: %w", initErr)
	}
	if client == nil {
		return nil, fmt.Errorf("Migration OCI client is not configured")
	}

	response, err := client.GetWorkRequest(ctx, databasemigrationsdk.GetWorkRequestRequest{
		WorkRequestId: common.String(strings.TrimSpace(workRequestID)),
	})
	if err != nil {
		return nil, err
	}
	return response.WorkRequest, nil
}

func resolveMigrationGeneratedWorkRequestAction(workRequest any) (string, error) {
	current, err := migrationWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}
	return string(current.OperationType), nil
}

func resolveMigrationGeneratedWorkRequestPhase(workRequest any) (shared.OSOKAsyncPhase, bool, error) {
	current, err := migrationWorkRequestFromAny(workRequest)
	if err != nil {
		return "", false, err
	}
	phase, ok := migrationWorkRequestPhaseFromOperationType(current.OperationType)
	return phase, ok, nil
}

func recoverMigrationIDFromGeneratedWorkRequest(
	_ *databasemigrationv1beta1.Migration,
	workRequest any,
	phase shared.OSOKAsyncPhase,
) (string, error) {
	current, err := migrationWorkRequestFromAny(workRequest)
	if err != nil {
		return "", err
	}

	action := migrationWorkRequestActionForPhase(phase)
	if id, ok := resolveMigrationIDFromResources(current.Resources, action, true); ok {
		return id, nil
	}
	if id, ok := resolveMigrationIDFromResources(current.Resources, action, false); ok {
		return id, nil
	}
	return "", fmt.Errorf("Migration work request %s does not expose a migration identifier", migrationStringValue(current.Id))
}

func migrationGeneratedWorkRequestMessage(phase shared.OSOKAsyncPhase, workRequest any) string {
	current, err := migrationWorkRequestFromAny(workRequest)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("Migration %s work request %s is %s", phase, migrationStringValue(current.Id), current.Status)
}

func migrationWorkRequestFromAny(workRequest any) (databasemigrationsdk.WorkRequest, error) {
	switch current := workRequest.(type) {
	case databasemigrationsdk.WorkRequest:
		return current, nil
	case *databasemigrationsdk.WorkRequest:
		if current == nil {
			return databasemigrationsdk.WorkRequest{}, fmt.Errorf("Migration work request is nil")
		}
		return *current, nil
	default:
		return databasemigrationsdk.WorkRequest{}, fmt.Errorf("unexpected Migration work request type %T", workRequest)
	}
}

func migrationWorkRequestPhaseFromOperationType(
	operationType databasemigrationsdk.OperationTypesEnum,
) (shared.OSOKAsyncPhase, bool) {
	switch operationType {
	case databasemigrationsdk.OperationTypesCreateMigration:
		return shared.OSOKAsyncPhaseCreate, true
	case databasemigrationsdk.OperationTypesUpdateMigration:
		return shared.OSOKAsyncPhaseUpdate, true
	case databasemigrationsdk.OperationTypesDeleteMigration:
		return shared.OSOKAsyncPhaseDelete, true
	default:
		return "", false
	}
}

func migrationWorkRequestActionForPhase(
	phase shared.OSOKAsyncPhase,
) databasemigrationsdk.WorkRequestResourceActionTypeEnum {
	switch phase {
	case shared.OSOKAsyncPhaseCreate:
		return databasemigrationsdk.WorkRequestResourceActionTypeCreated
	case shared.OSOKAsyncPhaseUpdate:
		return databasemigrationsdk.WorkRequestResourceActionTypeUpdated
	case shared.OSOKAsyncPhaseDelete:
		return databasemigrationsdk.WorkRequestResourceActionTypeDeleted
	default:
		return ""
	}
}

func resolveMigrationIDFromResources(
	resources []databasemigrationsdk.WorkRequestResource,
	action databasemigrationsdk.WorkRequestResourceActionTypeEnum,
	preferMigrationOnly bool,
) (string, bool) {
	var candidate string
	for _, resource := range resources {
		if action != "" && resource.ActionType != action {
			continue
		}
		if preferMigrationOnly && !isMigrationWorkRequestResource(resource) {
			continue
		}

		id := strings.TrimSpace(migrationStringValue(resource.Identifier))
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

func isMigrationWorkRequestResource(resource databasemigrationsdk.WorkRequestResource) bool {
	return normalizeMigrationWorkRequestToken(migrationStringValue(resource.EntityType)) == "migration"
}

func normalizeMigrationWorkRequestToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func migrationStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
