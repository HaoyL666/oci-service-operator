/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package migration

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	databasemigrationsdk "github.com/oracle/oci-go-sdk/v65/databasemigration"
	databasemigrationv1beta1 "github.com/oracle/oci-service-operator/api/databasemigration/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type fakeMigrationOCIClient struct {
	createFn      func(context.Context, databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error)
	getFn         func(context.Context, databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error)
	listFn        func(context.Context, databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error)
	updateFn      func(context.Context, databasemigrationsdk.UpdateMigrationRequest) (databasemigrationsdk.UpdateMigrationResponse, error)
	deleteFn      func(context.Context, databasemigrationsdk.DeleteMigrationRequest) (databasemigrationsdk.DeleteMigrationResponse, error)
	workRequestFn func(context.Context, databasemigrationsdk.GetWorkRequestRequest) (databasemigrationsdk.GetWorkRequestResponse, error)
}

func (f *fakeMigrationOCIClient) CreateMigration(
	ctx context.Context,
	req databasemigrationsdk.CreateMigrationRequest,
) (databasemigrationsdk.CreateMigrationResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return databasemigrationsdk.CreateMigrationResponse{}, nil
}

func (f *fakeMigrationOCIClient) GetMigration(
	ctx context.Context,
	req databasemigrationsdk.GetMigrationRequest,
) (databasemigrationsdk.GetMigrationResponse, error) {
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return databasemigrationsdk.GetMigrationResponse{}, nil
}

func (f *fakeMigrationOCIClient) ListMigrations(
	ctx context.Context,
	req databasemigrationsdk.ListMigrationsRequest,
) (databasemigrationsdk.ListMigrationsResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return databasemigrationsdk.ListMigrationsResponse{}, nil
}

func (f *fakeMigrationOCIClient) UpdateMigration(
	ctx context.Context,
	req databasemigrationsdk.UpdateMigrationRequest,
) (databasemigrationsdk.UpdateMigrationResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return databasemigrationsdk.UpdateMigrationResponse{}, nil
}

func (f *fakeMigrationOCIClient) DeleteMigration(
	ctx context.Context,
	req databasemigrationsdk.DeleteMigrationRequest,
) (databasemigrationsdk.DeleteMigrationResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return databasemigrationsdk.DeleteMigrationResponse{}, nil
}

func (f *fakeMigrationOCIClient) GetWorkRequest(
	ctx context.Context,
	req databasemigrationsdk.GetWorkRequestRequest,
) (databasemigrationsdk.GetWorkRequestResponse, error) {
	if f.workRequestFn != nil {
		return f.workRequestFn(ctx, req)
	}
	return databasemigrationsdk.GetWorkRequestResponse{}, nil
}

type migrationRequestBodyBuilder interface {
	HTTPRequest(
		method string,
		path string,
		binaryRequestBody *common.OCIReadSeekCloser,
		extraHeaders map[string]string,
	) (http.Request, error)
}

func newTestMigrationClient(fake *fakeMigrationOCIClient) MigrationServiceClient {
	return newMigrationServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: logr.Discard()},
		fake,
	)
}

func TestReviewedMigrationRuntimeSemanticsEncodesWorkRequestContract(t *testing.T) {
	t.Parallel()

	got := reviewedMigrationRuntimeSemantics()
	if got == nil {
		t.Fatal("reviewedMigrationRuntimeSemantics() = nil")
	}

	if got.FormalService != "databasemigration" {
		t.Fatalf("FormalService = %q, want databasemigration", got.FormalService)
	}
	if got.FormalSlug != "migration" {
		t.Fatalf("FormalSlug = %q, want migration", got.FormalSlug)
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
	assertMigrationStringSliceEqual(t, "Async.WorkRequest.Phases", got.Async.WorkRequest.Phases, []string{"create", "update", "delete"})
	assertMigrationStringSliceEqual(t, "Lifecycle.ProvisioningStates", got.Lifecycle.ProvisioningStates, []string{"ACCEPTED", "CREATING", "IN_PROGRESS", "WAITING"})
	assertMigrationStringSliceEqual(t, "Lifecycle.UpdatingStates", got.Lifecycle.UpdatingStates, []string{"UPDATING"})
	assertMigrationStringSliceEqual(t, "Lifecycle.ActiveStates", got.Lifecycle.ActiveStates, []string{"ACTIVE", "INACTIVE", "SUCCEEDED"})
	assertMigrationStringSliceEqual(t, "Delete.PendingStates", got.Delete.PendingStates, []string{"DELETING"})
	assertMigrationStringSliceEqual(t, "Delete.TerminalStates", got.Delete.TerminalStates, []string{"DELETED"})
	assertMigrationStringSliceEqual(t, "Mutation.ForceNew", got.Mutation.ForceNew, []string{"assessmentId", "compartmentId", "databaseCombination"})
	assertMigrationStringSliceContainsAll(t, "Mutation.Mutable", got.Mutation.Mutable, "displayName", "sourceDatabaseConnectionId", "targetDatabaseConnectionId", "type", "hubDetails")
	if got.List != nil {
		t.Fatalf("List = %#v, want nil generic list semantics because exact bind-before-create is custom", got.List)
	}
	if got.CreateFollowUp.Strategy != "GetWorkRequest -> GetMigration" {
		t.Fatalf("CreateFollowUp.Strategy = %q, want workrequest-backed follow-up", got.CreateFollowUp.Strategy)
	}
	if got.UpdateFollowUp.Strategy != "GetWorkRequest -> GetMigration" {
		t.Fatalf("UpdateFollowUp.Strategy = %q, want workrequest-backed follow-up", got.UpdateFollowUp.Strategy)
	}
	if got.DeleteFollowUp.Strategy != "GetWorkRequest -> GetMigration/ListMigrations confirm-delete" {
		t.Fatalf("DeleteFollowUp.Strategy = %q, want workrequest-backed confirm-delete", got.DeleteFollowUp.Strategy)
	}
	if len(got.AuxiliaryOperations) != 0 {
		t.Fatalf("AuxiliaryOperations = %#v, want none for published runtime", got.AuxiliaryOperations)
	}
}

func TestGuardMigrationExistingBeforeCreate(t *testing.T) {
	t.Parallel()

	resource := makeMySQLMigrationResource()
	resource.Spec.DisplayName = ""

	decision, err := guardMigrationExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardMigrationExistingBeforeCreate(empty displayName) error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionSkip {
		t.Fatalf("guardMigrationExistingBeforeCreate(empty displayName) = %q, want %q", decision, generatedruntime.ExistingBeforeCreateDecisionSkip)
	}

	resource.Spec.DisplayName = "mysql-migration"
	resource.Spec.SourceDatabaseConnectionId = ""
	decision, err = guardMigrationExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardMigrationExistingBeforeCreate(empty sourceDatabaseConnectionId) error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionSkip {
		t.Fatalf("guardMigrationExistingBeforeCreate(empty sourceDatabaseConnectionId) = %q, want %q", decision, generatedruntime.ExistingBeforeCreateDecisionSkip)
	}

	resource.Spec.SourceDatabaseConnectionId = "ocid1.connection.oc1..mysql-source"
	decision, err = guardMigrationExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardMigrationExistingBeforeCreate(valid identity) error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionAllow {
		t.Fatalf("guardMigrationExistingBeforeCreate(valid identity) = %q, want %q", decision, generatedruntime.ExistingBeforeCreateDecisionAllow)
	}
}

func TestBuildMigrationCreateDetailsUsesConcretePolymorphicBodies(t *testing.T) {
	t.Parallel()

	t.Run("MYSQL", func(t *testing.T) {
		t.Parallel()

		resource := makeMySQLMigrationResource()
		details, err := buildMigrationCreateDetails(context.Background(), resource, resource.Namespace)
		if err != nil {
			t.Fatalf("buildMigrationCreateDetails(MYSQL) error = %v", err)
		}

		createDetails, ok := details.(databasemigrationsdk.CreateMySqlMigrationDetails)
		if !ok {
			t.Fatalf("create body type = %T, want databasemigration.CreateMySqlMigrationDetails", details)
		}
		requireMigrationStringPtr(t, "assessmentId", createDetails.AssessmentId, resource.Spec.AssessmentId)
		requireMigrationStringPtr(t, "sourceDatabaseConnectionId", createDetails.SourceDatabaseConnectionId, resource.Spec.SourceDatabaseConnectionId)
		requireMigrationStringPtr(t, "targetDatabaseConnectionId", createDetails.TargetDatabaseConnectionId, resource.Spec.TargetDatabaseConnectionId)

		body := migrationSerializedRequestBody(t, databasemigrationsdk.CreateMigrationRequest{CreateMigrationDetails: details}, http.MethodPost, "/migrations")
		for _, want := range []string{
			`"databaseCombination":"MYSQL"`,
			`"type":"ONLINE"`,
			`"assessmentId":"ocid1.assessment.oc1..mysql"`,
			`"type":"OBJECT_STORAGE"`,
			`"bucketName":"mysql-dumps"`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("request body %s does not contain %s", body, want)
			}
		}
	})

	t.Run("ORACLE", func(t *testing.T) {
		t.Parallel()

		resource := makeOracleMigrationResource()
		details, err := buildMigrationCreateDetails(context.Background(), resource, resource.Namespace)
		if err != nil {
			t.Fatalf("buildMigrationCreateDetails(ORACLE) error = %v", err)
		}

		createDetails, ok := details.(databasemigrationsdk.CreateOracleMigrationDetails)
		if !ok {
			t.Fatalf("create body type = %T, want databasemigration.CreateOracleMigrationDetails", details)
		}
		requireMigrationStringPtr(t, "sourceContainerDatabaseConnectionId", createDetails.SourceContainerDatabaseConnectionId, resource.Spec.SourceContainerDatabaseConnectionId)
		requireMigrationStringPtr(t, "sourceStandbyDatabaseConnectionId", createDetails.SourceStandbyDatabaseConnectionId, resource.Spec.SourceStandbyDatabaseConnectionId)
		if len(createDetails.AdvancedParameters) != 1 {
			t.Fatalf("advancedParameters = %#v, want one projected Oracle advanced parameter", createDetails.AdvancedParameters)
		}

		body := migrationSerializedRequestBody(t, databasemigrationsdk.CreateMigrationRequest{CreateMigrationDetails: details}, http.MethodPost, "/migrations")
		for _, want := range []string{
			`"databaseCombination":"ORACLE"`,
			`"type":"OFFLINE"`,
			`"advancedParameters":[{"dataType":"STRING","name":"parallelism","value":"8"}]`,
			`"sourceContainerDatabaseConnectionId":"ocid1.connection.oc1..oracle-cdb"`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("request body %s does not contain %s", body, want)
			}
		}
	})
}

func TestBuildMigrationCreateDetailsRejectsUnsupportedSpecSurface(t *testing.T) {
	t.Parallel()

	t.Run("MYSQL rejects oracle-only fields", func(t *testing.T) {
		t.Parallel()

		resource := makeMySQLMigrationResource()
		resource.Spec.AdvancedParameters = []databasemigrationv1beta1.MigrationAdvancedParameter{
			{Name: "parallelism", DataType: "STRING", Value: "8"},
		}
		if _, err := buildMigrationCreateDetails(context.Background(), resource, resource.Namespace); err == nil {
			t.Fatal("buildMigrationCreateDetails(MYSQL with advancedParameters) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "spec.advancedParameters") {
			t.Fatalf("buildMigrationCreateDetails(MYSQL with advancedParameters) error = %v, want spec.advancedParameters rejection", err)
		}
	})

	t.Run("ORACLE rejects mysql-only initialLoadSettings fields", func(t *testing.T) {
		t.Parallel()

		resource := makeOracleMigrationResource()
		resource.Spec.InitialLoadSettings = databasemigrationv1beta1.MigrationInitialLoadSettings{
			JobMode:      "FULL",
			IsConsistent: true,
		}
		if _, err := buildMigrationCreateDetails(context.Background(), resource, resource.Namespace); err == nil {
			t.Fatal("buildMigrationCreateDetails(ORACLE with MySQL-only initialLoadSettings) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "spec.initialLoadSettings.isConsistent") {
			t.Fatalf("buildMigrationCreateDetails(ORACLE with MySQL-only initialLoadSettings) error = %v, want spec.initialLoadSettings.isConsistent rejection", err)
		}
	})

	t.Run("rejects unsupported dataTransferMediumDetails type", func(t *testing.T) {
		t.Parallel()

		resource := makeMySQLMigrationResource()
		resource.Spec.DataTransferMediumDetails.Type = "AWS_S3"
		if _, err := buildMigrationCreateDetails(context.Background(), resource, resource.Namespace); err == nil {
			t.Fatal("buildMigrationCreateDetails(unsupported data transfer type) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "spec.dataTransferMediumDetails.type") {
			t.Fatalf("buildMigrationCreateDetails(unsupported data transfer type) error = %v, want spec.dataTransferMediumDetails.type rejection", err)
		}
	})
}

func TestBuildMigrationUpdateDetailsUsesConcretePolymorphicBody(t *testing.T) {
	t.Parallel()

	resource := makeOracleMigrationResource()
	resource.Spec.DisplayName = "oracle-migration-updated"
	resource.Spec.SourceContainerDatabaseConnectionId = "ocid1.connection.oc1..oracle-cdb-updated"
	resource.Spec.Description = ""
	resource.Spec.FreeformTags = map[string]string{}

	details, updateNeeded, err := buildMigrationUpdateDetails(
		context.Background(),
		resource,
		resource.Namespace,
		databasemigrationsdk.GetMigrationResponse{
			Migration: makeOracleSDKMigration(
				"ocid1.migration.oc1..oracle",
				makeOracleMigrationResource(),
				databasemigrationsdk.MigrationLifecycleStatesActive,
			),
		},
	)
	if err != nil {
		t.Fatalf("buildMigrationUpdateDetails() error = %v", err)
	}
	if !updateNeeded {
		t.Fatal("buildMigrationUpdateDetails() updateNeeded = false, want true after mutable drift")
	}

	updateDetails, ok := details.(databasemigrationsdk.UpdateOracleMigrationDetails)
	if !ok {
		t.Fatalf("update body type = %T, want databasemigration.UpdateOracleMigrationDetails", details)
	}
	requireMigrationStringPtr(t, "displayName", updateDetails.DisplayName, resource.Spec.DisplayName)
	requireMigrationStringPtr(t, "sourceContainerDatabaseConnectionId", updateDetails.SourceContainerDatabaseConnectionId, resource.Spec.SourceContainerDatabaseConnectionId)

	body := migrationSerializedRequestBody(
		t,
		databasemigrationsdk.UpdateMigrationRequest{
			MigrationId:            common.String("ocid1.migration.oc1..oracle"),
			UpdateMigrationDetails: details,
		},
		http.MethodPut,
		"/migrations/ocid1.migration.oc1..oracle",
	)
	for _, want := range []string{
		`"databaseCombination":"ORACLE"`,
		`"displayName":"oracle-migration-updated"`,
		`"sourceContainerDatabaseConnectionId":"ocid1.connection.oc1..oracle-cdb-updated"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body %s does not contain %s", body, want)
		}
	}
}

func TestBuildMigrationUpdateDetailsIgnoresWriteOnlyHubPasswordParity(t *testing.T) {
	t.Parallel()

	resource := makeMySQLMigrationResource()
	details, updateNeeded, err := buildMigrationUpdateDetails(
		context.Background(),
		resource,
		resource.Namespace,
		databasemigrationsdk.GetMigrationResponse{
			Migration: makeMySQLSDKMigration(
				"ocid1.migration.oc1..mysql",
				resource,
				databasemigrationsdk.MigrationLifecycleStatesActive,
			),
		},
	)
	if err != nil {
		t.Fatalf("buildMigrationUpdateDetails() error = %v", err)
	}
	if updateNeeded {
		t.Fatalf("buildMigrationUpdateDetails() updateNeeded = true, want false when only write-only hub password differs; details=%#v", details)
	}
}

func TestLookupExistingMigrationReturnsExactMatchOnly(t *testing.T) {
	t.Parallel()

	t.Run("returns exact match", func(t *testing.T) {
		t.Parallel()

		resource := makeMySQLMigrationResource()
		identity, err := resolveMigrationIdentity(resource)
		if err != nil {
			t.Fatalf("resolveMigrationIdentity() error = %v", err)
		}

		var getRequests []string
		response, err := lookupExistingMigration(
			context.Background(),
			&fakeMigrationOCIClient{
				listFn: func(_ context.Context, req databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error) {
					requireMigrationStringPtr(t, "list compartmentId", req.CompartmentId, resource.Spec.CompartmentId)
					requireMigrationStringPtr(t, "list displayName", req.DisplayName, resource.Spec.DisplayName)
					return databasemigrationsdk.ListMigrationsResponse{
						MigrationCollection: databasemigrationsdk.MigrationCollection{
							Items: []databasemigrationsdk.MigrationSummary{
								makeMySQLMigrationSummary("ocid1.migration.oc1..mysql", resource, databasemigrationsdk.MigrationLifecycleStatesActive),
							},
						},
					}, nil
				},
				getFn: func(_ context.Context, req databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error) {
					getRequests = append(getRequests, migrationStringValue(req.MigrationId))
					return databasemigrationsdk.GetMigrationResponse{
						Migration: makeMySQLSDKMigration("ocid1.migration.oc1..mysql", resource, databasemigrationsdk.MigrationLifecycleStatesActive),
					}, nil
				},
			},
			nil,
			resource,
			identity,
		)
		if err != nil {
			t.Fatalf("lookupExistingMigration() error = %v", err)
		}
		if len(getRequests) != 1 || getRequests[0] != "ocid1.migration.oc1..mysql" {
			t.Fatalf("GetMigration requests = %#v, want exact candidate fetch", getRequests)
		}
		got, ok := response.(databasemigrationsdk.GetMigrationResponse)
		if !ok {
			t.Fatalf("lookupExistingMigration() response type = %T, want databasemigration.GetMigrationResponse", response)
		}
		if migrationIDFromAny(got.Migration) != "ocid1.migration.oc1..mysql" {
			t.Fatalf("lookupExistingMigration() returned id %q, want ocid1.migration.oc1..mysql", migrationIDFromAny(got.Migration))
		}
	})

	t.Run("does not bind non-identical candidate", func(t *testing.T) {
		t.Parallel()

		resource := makeMySQLMigrationResource()
		identity, err := resolveMigrationIdentity(resource)
		if err != nil {
			t.Fatalf("resolveMigrationIdentity() error = %v", err)
		}

		response, err := lookupExistingMigration(
			context.Background(),
			&fakeMigrationOCIClient{
				listFn: func(_ context.Context, _ databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error) {
					return databasemigrationsdk.ListMigrationsResponse{
						MigrationCollection: databasemigrationsdk.MigrationCollection{
							Items: []databasemigrationsdk.MigrationSummary{
								makeMySQLMigrationSummary("ocid1.migration.oc1..other", resource, databasemigrationsdk.MigrationLifecycleStatesActive),
							},
						},
					}, nil
				},
				getFn: func(_ context.Context, _ databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error) {
					mismatched := makeMySQLMigrationResource()
					mismatched.Spec.Type = "OFFLINE"
					return databasemigrationsdk.GetMigrationResponse{
						Migration: makeMySQLSDKMigration("ocid1.migration.oc1..other", mismatched, databasemigrationsdk.MigrationLifecycleStatesActive),
					}, nil
				},
			},
			nil,
			resource,
			identity,
		)
		if err != nil {
			t.Fatalf("lookupExistingMigration() error = %v", err)
		}
		if response != nil {
			t.Fatalf("lookupExistingMigration() = %#v, want nil for non-identical candidate", response)
		}
	})
}

func TestRecoverMigrationIDFromGeneratedWorkRequest(t *testing.T) {
	t.Parallel()

	workRequest := databasemigrationsdk.WorkRequest{
		Id:            common.String("wr-migration-create"),
		OperationType: databasemigrationsdk.OperationTypesCreateMigration,
		Status:        databasemigrationsdk.OperationStatusSucceeded,
		Resources: []databasemigrationsdk.WorkRequestResource{
			{
				ActionType: databasemigrationsdk.WorkRequestResourceActionTypeRelated,
				EntityType: common.String("assessment"),
				Identifier: common.String("ocid1.assessment.oc1..ignore"),
			},
			{
				ActionType: databasemigrationsdk.WorkRequestResourceActionTypeCreated,
				EntityType: common.String("Migration"),
				Identifier: common.String("ocid1.migration.oc1..created"),
			},
		},
	}

	id, err := recoverMigrationIDFromGeneratedWorkRequest(nil, workRequest, shared.OSOKAsyncPhaseCreate)
	if err != nil {
		t.Fatalf("recoverMigrationIDFromGeneratedWorkRequest() error = %v", err)
	}
	if id != "ocid1.migration.oc1..created" {
		t.Fatalf("recoverMigrationIDFromGeneratedWorkRequest() = %q, want %q", id, "ocid1.migration.oc1..created")
	}
}

func TestMigrationServiceClientCreatesAndResumesWorkRequest(t *testing.T) {
	t.Parallel()

	const (
		createdID     = "ocid1.migration.oc1..created"
		workRequestID = "wr-migration-create"
	)

	resource := makeMySQLMigrationResource()
	workRequests := map[string]databasemigrationsdk.WorkRequest{
		workRequestID: makeMigrationWorkRequest(
			workRequestID,
			databasemigrationsdk.OperationTypesCreateMigration,
			databasemigrationsdk.OperationStatusInProgress,
			databasemigrationsdk.WorkRequestResourceActionTypeInProgress,
			"",
		),
	}

	var createRequest databasemigrationsdk.CreateMigrationRequest
	var listRequest databasemigrationsdk.ListMigrationsRequest
	getCalls := 0

	client := newTestMigrationClient(&fakeMigrationOCIClient{
		listFn: func(_ context.Context, req databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error) {
			listRequest = req
			return databasemigrationsdk.ListMigrationsResponse{}, nil
		},
		createFn: func(_ context.Context, req databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error) {
			createRequest = req
			return databasemigrationsdk.CreateMigrationResponse{
				OpcWorkRequestId: common.String(workRequestID),
				OpcRequestId:     common.String("opc-create-migration"),
			}, nil
		},
		workRequestFn: func(_ context.Context, req databasemigrationsdk.GetWorkRequestRequest) (databasemigrationsdk.GetWorkRequestResponse, error) {
			requireMigrationStringPtr(t, "workRequestId", req.WorkRequestId, workRequestID)
			return databasemigrationsdk.GetWorkRequestResponse{WorkRequest: workRequests[workRequestID]}, nil
		},
		getFn: func(_ context.Context, req databasemigrationsdk.GetMigrationRequest) (databasemigrationsdk.GetMigrationResponse, error) {
			getCalls++
			requireMigrationStringPtr(t, "get migrationId", req.MigrationId, createdID)
			return databasemigrationsdk.GetMigrationResponse{
				Migration: makeMySQLSDKMigration(createdID, resource, databasemigrationsdk.MigrationLifecycleStatesSucceeded),
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
	requireMigrationStringPtr(t, "list compartmentId", listRequest.CompartmentId, resource.Spec.CompartmentId)
	requireMigrationStringPtr(t, "list displayName", listRequest.DisplayName, resource.Spec.DisplayName)
	if listRequest.LifecycleState != "" {
		t.Fatalf("list lifecycleState = %q, want empty reviewed lookup filter", listRequest.LifecycleState)
	}
	if listRequest.LifecycleDetails != "" {
		t.Fatalf("list lifecycleDetails = %q, want empty reviewed lookup filter", listRequest.LifecycleDetails)
	}
	if getCalls != 0 {
		t.Fatalf("GetMigration() calls = %d, want 0 while work request is pending", getCalls)
	}
	requireMigrationAsyncCurrent(t, resource, shared.OSOKAsyncPhaseCreate, workRequestID, shared.OSOKAsyncClassPending)
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-create-migration" {
		t.Fatalf("status.opcRequestId = %q, want opc-create-migration", got)
	}

	body := migrationSerializedRequestBody(t, createRequest, http.MethodPost, "/migrations")
	for _, want := range []string{
		`"databaseCombination":"MYSQL"`,
		`"type":"ONLINE"`,
		`"sourceDatabaseConnectionId":"ocid1.connection.oc1..mysql-source"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("create request body %s does not contain %s", body, want)
		}
	}

	workRequests[workRequestID] = makeMigrationWorkRequest(
		workRequestID,
		databasemigrationsdk.OperationTypesCreateMigration,
		databasemigrationsdk.OperationStatusSucceeded,
		databasemigrationsdk.WorkRequestResourceActionTypeCreated,
		createdID,
	)

	response, err = client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() after work request success error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() after work request success response = %#v, want converged success", response)
	}
	if getCalls != 1 {
		t.Fatalf("GetMigration() calls = %d, want 1 follow-up read", getCalls)
	}
	if got := resource.Status.Id; got != createdID {
		t.Fatalf("status.id = %q, want %q", got, createdID)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != createdID {
		t.Fatalf("status.ocid = %q, want %q", got, createdID)
	}
	if got := resource.Status.DatabaseCombination; got != "MYSQL" {
		t.Fatalf("status.databaseCombination = %q, want MYSQL", got)
	}
	if got := resource.Status.LifecycleState; got != string(databasemigrationsdk.MigrationLifecycleStatesSucceeded) {
		t.Fatalf("status.lifecycleState = %q, want %q", got, databasemigrationsdk.MigrationLifecycleStatesSucceeded)
	}
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.async.current = %#v, want nil after successful reconciliation", resource.Status.OsokStatus.Async.Current)
	}
}

func TestMigrationServiceClientRejectsOutOfScopeObjectHelpersBeforeOCI(t *testing.T) {
	t.Parallel()

	resource := makeOracleMigrationResource()
	resource.Spec.ExcludeObjects = []databasemigrationv1beta1.MigrationExcludeObject{
		{Schema: "HR", ObjectName: "EMPLOYEES"},
	}

	createCalled := false
	client := newTestMigrationClient(&fakeMigrationOCIClient{
		listFn: func(_ context.Context, _ databasemigrationsdk.ListMigrationsRequest) (databasemigrationsdk.ListMigrationsResponse, error) {
			return databasemigrationsdk.ListMigrationsResponse{}, nil
		},
		createFn: func(_ context.Context, _ databasemigrationsdk.CreateMigrationRequest) (databasemigrationsdk.CreateMigrationResponse, error) {
			createCalled = true
			return databasemigrationsdk.CreateMigrationResponse{}, nil
		},
	})

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want out-of-scope helper rejection")
	} else if !strings.Contains(err.Error(), "spec.excludeObjects") {
		t.Fatalf("CreateOrUpdate() error = %v, want spec.excludeObjects rejection", err)
	}
	if createCalled {
		t.Fatal("CreateMigration() called after out-of-scope helper rejection")
	}
}

func makeMySQLMigrationResource() *databasemigrationv1beta1.Migration {
	return &databasemigrationv1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-migration",
			Namespace: "default",
		},
		Spec: databasemigrationv1beta1.MigrationSpec{
			DisplayName:                "mysql-migration",
			CompartmentId:              "ocid1.compartment.oc1..mysql",
			Type:                       "ONLINE",
			Description:                "mysql migration",
			SourceDatabaseConnectionId: "ocid1.connection.oc1..mysql-source",
			TargetDatabaseConnectionId: "ocid1.connection.oc1..mysql-target",
			AssessmentId:               "ocid1.assessment.oc1..mysql",
			DatabaseCombination:        "MYSQL",
			DataTransferMediumDetails: databasemigrationv1beta1.MigrationDataTransferMediumDetails{
				Type: "OBJECT_STORAGE",
				ObjectStorageBucket: databasemigrationv1beta1.MigrationDataTransferMediumDetailsObjectStorageBucket{
					NamespaceName: "migration-ns",
					BucketName:    "mysql-dumps",
				},
			},
			HubDetails: databasemigrationv1beta1.MigrationHubDetails{
				RestAdminCredentials: databasemigrationv1beta1.MigrationHubDetailsRestAdminCredentials{
					Username: "ggadmin",
					Password: "HubSecret123",
				},
				Url:           "https://gg.mysql.example.com",
				VaultId:       "ocid1.vault.oc1..mysql",
				KeyId:         "ocid1.key.oc1..mysql",
				AcceptableLag: 30,
			},
			FreeformTags: map[string]string{"env": "test"},
			DefinedTags:  map[string]shared.MapValue{"Operations": {"CostCenter": "42"}},
		},
	}
}

func makeOracleMigrationResource() *databasemigrationv1beta1.Migration {
	return &databasemigrationv1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oracle-migration",
			Namespace: "default",
		},
		Spec: databasemigrationv1beta1.MigrationSpec{
			DisplayName:                         "oracle-migration",
			CompartmentId:                       "ocid1.compartment.oc1..oracle",
			Type:                                "OFFLINE",
			Description:                         "oracle migration",
			SourceDatabaseConnectionId:          "ocid1.connection.oc1..oracle-source",
			TargetDatabaseConnectionId:          "ocid1.connection.oc1..oracle-target",
			AssessmentId:                        "ocid1.assessment.oc1..oracle",
			DatabaseCombination:                 "ORACLE",
			SourceContainerDatabaseConnectionId: "ocid1.connection.oc1..oracle-cdb",
			SourceStandbyDatabaseConnectionId:   "ocid1.connection.oc1..oracle-standby",
			AdvancedParameters: []databasemigrationv1beta1.MigrationAdvancedParameter{
				{Name: "parallelism", DataType: "STRING", Value: "8"},
			},
			DataTransferMediumDetails: databasemigrationv1beta1.MigrationDataTransferMediumDetails{
				Type: "OBJECT_STORAGE",
				ObjectStorageBucket: databasemigrationv1beta1.MigrationDataTransferMediumDetailsObjectStorageBucket{
					NamespaceName: "migration-ns",
					BucketName:    "oracle-dumps",
				},
			},
			HubDetails: databasemigrationv1beta1.MigrationHubDetails{
				RestAdminCredentials: databasemigrationv1beta1.MigrationHubDetailsRestAdminCredentials{
					Username: "ggadmin",
					Password: "OracleHubSecret123",
				},
				Url:           "https://gg.oracle.example.com",
				VaultId:       "ocid1.vault.oc1..oracle",
				KeyId:         "ocid1.key.oc1..oracle",
				AcceptableLag: 45,
			},
			FreeformTags: map[string]string{"env": "test"},
			DefinedTags:  map[string]shared.MapValue{"Operations": {"CostCenter": "84"}},
		},
	}
}

func makeMySQLSDKMigration(
	id string,
	resource *databasemigrationv1beta1.Migration,
	state databasemigrationsdk.MigrationLifecycleStatesEnum,
) databasemigrationsdk.MySqlMigration {
	now := &common.SDKTime{Time: time.Unix(1713240000, 0).UTC()}
	return databasemigrationsdk.MySqlMigration{
		Id:                         common.String(id),
		DisplayName:                common.String(resource.Spec.DisplayName),
		CompartmentId:              common.String(resource.Spec.CompartmentId),
		TimeCreated:                now,
		TimeUpdated:                now,
		TimeLastMigration:          now,
		Description:                common.String(resource.Spec.Description),
		AssessmentId:               common.String(resource.Spec.AssessmentId),
		SourceDatabaseConnectionId: common.String(resource.Spec.SourceDatabaseConnectionId),
		TargetDatabaseConnectionId: common.String(resource.Spec.TargetDatabaseConnectionId),
		FreeformTags:               map[string]string{"env": "test"},
		DefinedTags:                map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		DataTransferMediumDetails: databasemigrationsdk.MySqlObjectStorageDataTransferMediumDetails{
			ObjectStorageBucket: &databasemigrationsdk.ObjectStoreBucket{
				NamespaceName: common.String(resource.Spec.DataTransferMediumDetails.ObjectStorageBucket.NamespaceName),
				BucketName:    common.String(resource.Spec.DataTransferMediumDetails.ObjectStorageBucket.BucketName),
			},
		},
		HubDetails: &databasemigrationsdk.GoldenGateHubDetails{
			RestAdminCredentials: &databasemigrationsdk.AdminCredentials{
				Username: common.String(resource.Spec.HubDetails.RestAdminCredentials.Username),
			},
			Url:           common.String(resource.Spec.HubDetails.Url),
			VaultId:       common.String(resource.Spec.HubDetails.VaultId),
			KeyId:         common.String(resource.Spec.HubDetails.KeyId),
			AcceptableLag: common.Int(resource.Spec.HubDetails.AcceptableLag),
		},
		Type:             migrationTypeEnumForSpec(resource.Spec.Type),
		WaitAfter:        databasemigrationsdk.OdmsJobPhasesOdmsValidate,
		LifecycleState:   state,
		LifecycleDetails: databasemigrationsdk.MigrationStatusReady,
	}
}

func makeOracleSDKMigration(
	id string,
	resource *databasemigrationv1beta1.Migration,
	state databasemigrationsdk.MigrationLifecycleStatesEnum,
) databasemigrationsdk.OracleMigration {
	now := &common.SDKTime{Time: time.Unix(1713240000, 0).UTC()}
	return databasemigrationsdk.OracleMigration{
		Id:                                  common.String(id),
		DisplayName:                         common.String(resource.Spec.DisplayName),
		CompartmentId:                       common.String(resource.Spec.CompartmentId),
		TimeCreated:                         now,
		TimeUpdated:                         now,
		TimeLastMigration:                   now,
		Description:                         common.String(resource.Spec.Description),
		AssessmentId:                        common.String(resource.Spec.AssessmentId),
		SourceDatabaseConnectionId:          common.String(resource.Spec.SourceDatabaseConnectionId),
		TargetDatabaseConnectionId:          common.String(resource.Spec.TargetDatabaseConnectionId),
		SourceContainerDatabaseConnectionId: common.String(resource.Spec.SourceContainerDatabaseConnectionId),
		SourceStandbyDatabaseConnectionId:   common.String(resource.Spec.SourceStandbyDatabaseConnectionId),
		AdvancedParameters: []databasemigrationsdk.MigrationParameterDetails{
			{
				Name:     common.String("parallelism"),
				DataType: databasemigrationsdk.AdvancedParameterDataTypesString,
				Value:    common.String("8"),
			},
		},
		FreeformTags: map[string]string{"env": "test"},
		DefinedTags:  map[string]map[string]interface{}{"Operations": {"CostCenter": "84"}},
		DataTransferMediumDetails: databasemigrationsdk.OracleObjectStorageDataTransferMediumDetails{
			ObjectStorageBucket: &databasemigrationsdk.ObjectStoreBucket{
				NamespaceName: common.String(resource.Spec.DataTransferMediumDetails.ObjectStorageBucket.NamespaceName),
				BucketName:    common.String(resource.Spec.DataTransferMediumDetails.ObjectStorageBucket.BucketName),
			},
		},
		HubDetails: &databasemigrationsdk.GoldenGateHubDetails{
			RestAdminCredentials: &databasemigrationsdk.AdminCredentials{
				Username: common.String(resource.Spec.HubDetails.RestAdminCredentials.Username),
			},
			Url:           common.String(resource.Spec.HubDetails.Url),
			VaultId:       common.String(resource.Spec.HubDetails.VaultId),
			KeyId:         common.String(resource.Spec.HubDetails.KeyId),
			AcceptableLag: common.Int(resource.Spec.HubDetails.AcceptableLag),
		},
		Type:             migrationTypeEnumForSpec(resource.Spec.Type),
		WaitAfter:        databasemigrationsdk.OdmsJobPhasesOdmsValidate,
		LifecycleState:   state,
		LifecycleDetails: databasemigrationsdk.MigrationStatusDone,
	}
}

func makeMySQLMigrationSummary(
	id string,
	resource *databasemigrationv1beta1.Migration,
	state databasemigrationsdk.MigrationLifecycleStatesEnum,
) databasemigrationsdk.MySqlMigrationSummary {
	now := &common.SDKTime{Time: time.Unix(1713240000, 0).UTC()}
	return databasemigrationsdk.MySqlMigrationSummary{
		Id:                         common.String(id),
		DisplayName:                common.String(resource.Spec.DisplayName),
		CompartmentId:              common.String(resource.Spec.CompartmentId),
		Type:                       migrationTypeEnumForSpec(resource.Spec.Type),
		SourceDatabaseConnectionId: common.String(resource.Spec.SourceDatabaseConnectionId),
		TargetDatabaseConnectionId: common.String(resource.Spec.TargetDatabaseConnectionId),
		AssessmentId:               common.String(resource.Spec.AssessmentId),
		TimeCreated:                now,
		TimeUpdated:                now,
		TimeLastMigration:          now,
		FreeformTags:               map[string]string{"env": "test"},
		DefinedTags:                map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		LifecycleState:             state,
		LifecycleDetails:           databasemigrationsdk.MigrationStatusReady,
	}
}

func migrationTypeEnumForSpec(raw string) databasemigrationsdk.MigrationTypesEnum {
	migrationType, ok := databasemigrationsdk.GetMappingMigrationTypesEnum(raw)
	if !ok {
		return databasemigrationsdk.MigrationTypesEnum(raw)
	}
	return migrationType
}

func makeMigrationWorkRequest(
	id string,
	operation databasemigrationsdk.OperationTypesEnum,
	status databasemigrationsdk.OperationStatusEnum,
	action databasemigrationsdk.WorkRequestResourceActionTypeEnum,
	resourceID string,
) databasemigrationsdk.WorkRequest {
	now := &common.SDKTime{Time: time.Unix(1713240000, 0).UTC()}
	resources := []databasemigrationsdk.WorkRequestResource{
		{
			ActionType: action,
			EntityType: common.String("Migration"),
			Identifier: common.String(resourceID),
		},
	}
	return databasemigrationsdk.WorkRequest{
		Id:              common.String(id),
		CompartmentId:   common.String("ocid1.compartment.oc1..workrequest"),
		OperationType:   operation,
		Status:          status,
		Resources:       resources,
		PercentComplete: common.Float32(25),
		TimeAccepted:    now,
	}
}

func migrationSerializedRequestBody(
	t *testing.T,
	request migrationRequestBodyBuilder,
	method string,
	path string,
) string {
	t.Helper()

	httpRequest, err := request.HTTPRequest(method, path, nil, nil)
	if err != nil {
		t.Fatalf("HTTPRequest() error = %v", err)
	}

	body, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		t.Fatalf("ReadAll(request body) error = %v", err)
	}
	return string(body)
}

func migrationIDFromAny(payload any) string {
	switch current := payload.(type) {
	case databasemigrationsdk.MySqlMigration:
		return migrationStringValue(current.Id)
	case databasemigrationsdk.OracleMigration:
		return migrationStringValue(current.Id)
	case databasemigrationsdk.Migration:
		if current == nil {
			return ""
		}
		return migrationStringValue(current.GetId())
	default:
		return ""
	}
}

func requireMigrationStringPtr(t *testing.T, field string, actual *string, want string) {
	t.Helper()
	if actual == nil {
		t.Fatalf("%s = nil, want %q", field, want)
	}
	if *actual != want {
		t.Fatalf("%s = %q, want %q", field, *actual, want)
	}
}

func requireMigrationAsyncCurrent(
	t *testing.T,
	resource *databasemigrationv1beta1.Migration,
	phase shared.OSOKAsyncPhase,
	workRequestID string,
	class shared.OSOKAsyncNormalizedClass,
) {
	t.Helper()

	current := resource.Status.OsokStatus.Async.Current
	if current == nil {
		t.Fatal("status.async.current = nil")
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

func assertMigrationStringSliceEqual(t *testing.T, field string, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", field, got, want)
	}
}

func assertMigrationStringSliceContainsAll(t *testing.T, field string, got []string, want ...string) {
	t.Helper()
	for _, candidate := range want {
		if !containsMigrationString(got, candidate) {
			t.Fatalf("%s = %#v, want %q included", field, got, candidate)
		}
	}
}

func containsMigrationString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
