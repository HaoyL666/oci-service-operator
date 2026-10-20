/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package distributeddatabase

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	distributeddatabasesdk "github.com/oracle/oci-go-sdk/v65/distributeddatabase"
	distributeddatabasev1beta1 "github.com/oracle/oci-service-operator/api/distributeddatabase/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: v65.127 catalog/replica SDK types, generated manager, and typed OCI mock lifecycle.
func TestMockIntegrationDistributedDatabaseLifecycleCRUD(t *testing.T) {
	t.Parallel()

	resource := newTestDistributedDatabaseResource()
	resource.Namespace = "default"
	resource.Name = "mock-distributeddatabase"
	ocimock.InitializeResource(resource, "mock-distributeddatabase")
	updatedSpec := resource.Spec
	updatedSpec.DisplayName = "ddb-runtime-updated"

	createRequest, err := buildDistributedDatabaseCreateBody(context.Background(), nil, resource, resource.Namespace)
	if err != nil {
		t.Fatalf("buildDistributedDatabaseCreateBody() error = %v", err)
	}
	createdState := observedDistributedDatabaseFromSpec("<ocid:10>", resource.Spec, distributeddatabasesdk.DistributedDatabaseLifecycleStateActive)
	createdState.CatalogDetails = []distributeddatabasesdk.DistributedDatabaseCatalog{distributedDatabaseCatalogStatus()}
	updatedState := observedDistributedDatabaseFromSpec("<ocid:10>", updatedSpec, distributeddatabasesdk.DistributedDatabaseLifecycleStateActive)
	updatedState.CatalogDetails = []distributeddatabasesdk.DistributedDatabaseCatalog{distributedDatabaseCatalogStatus()}
	updateRequest := distributeddatabasesdk.UpdateDistributedDatabaseDetails{DisplayName: common.String(updatedSpec.DisplayName)}

	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[
		distributeddatabasesdk.DistributedDatabase,
		distributeddatabasesdk.CreateDistributedDatabaseDetails,
		distributeddatabasesdk.UpdateDistributedDatabaseDetails,
	]{
		CollectionPath:     "/20250101/distributedDatabases",
		ItemPath:           "/20250101/distributedDatabases/<ocid:10>",
		Operations:         []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreateRequest:      &createRequest,
		CreatedState:       &createdState,
		ListShape:          ocimock.ListShapeItems,
		UpdateRequest:      &updateRequest,
		UpdatedState:       &updatedState,
		DeletedReadStates:  ocimock.LifecycleStates(t, updatedState, "DELETING"),
		CreatedReadStates:  append(ocimock.LifecycleStates(t, createdState, "CREATING"), createdState),
		UpdatedReadStates:  append(ocimock.LifecycleStates(t, updatedState, "UPDATING"), updatedState),
		DeleteEndsNotFound: true,
		RequireCreateRead:  true,
		RequireUpdateRead:  true,
		RequireDeleteRead:  true,
		CreateStatus:       201,
		UpdateStatus:       200,
		DeleteStatus:       204,
		NotFoundCode:       "NotFound",
		ValidateCreate: func(request ocimock.Request, _ distributeddatabasesdk.CreateDistributedDatabaseDetails) error {
			if request.Header.Get("opc-retry-token") == "" {
				return fmt.Errorf("create retry token is empty")
			}
			return nil
		},
		ValidateDelete: func(request ocimock.Request, _ distributeddatabasesdk.DistributedDatabase) error {
			if len(request.Body) != 0 {
				return fmt.Errorf("delete body = %s", request.Body)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://oci.mock.invalid", BasePath: "20250101", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DistributedDatabase OCI mock: %v", err)
		}
	})

	sdkClient := distributeddatabasesdk.DistributedDbServiceClient{BaseClient: session.BaseClient()}
	manager := &DistributedDatabaseServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDistributedDatabaseDefaultRuntimeHooks(sdkClient)
	applyDistributedDatabaseRuntimeHooks(manager, &hooks)
	client := wrapDistributedDatabaseGeneratedClient(hooks, defaultDistributedDatabaseServiceClient{ServiceClient: generatedruntime.NewServiceClient[*distributeddatabasev1beta1.DistributedDatabase](buildDistributedDatabaseGeneratedRuntimeConfig(manager, hooks))})

	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*distributeddatabasev1beta1.DistributedDatabase]{
		Resource:      resource,
		Client:        client,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *distributeddatabasev1beta1.DistributedDatabase) error {
			if current.Status.Id != "<ocid:10>" ||
				string(current.Status.OsokStatus.Ocid) != "<ocid:10>" ||
				current.Status.LifecycleState != "ACTIVE" ||
				current.Status.DisplayName != current.Spec.DisplayName ||
				current.Status.ShardingMethod != "USER" ||
				current.Status.DbDeploymentType != "EXADB" ||
				len(current.Status.CatalogDetails) != 1 ||
				current.Status.CatalogDetails[0].OriginalReplica.Name != "catalog" ||
				current.Status.CatalogDetails[0].OriginalReplica.Database.Source != "XS_NEW_VAULT_AND_CLUSTER" ||
				current.Status.CatalogDetails[0].OriginalReplica.Database.AvailabilityDomain != "Uocm:PHX-AD-1" {
				return fmt.Errorf("created DistributedDatabase status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *distributeddatabasev1beta1.DistributedDatabase) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *distributeddatabasev1beta1.DistributedDatabase) error {
			if current.Status.Id != "<ocid:10>" ||
				string(current.Status.OsokStatus.Ocid) != "<ocid:10>" ||
				current.Status.LifecycleState != "ACTIVE" ||
				current.Status.DisplayName != current.Spec.DisplayName {
				return fmt.Errorf("updated DistributedDatabase status = %+v", current.Status)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func distributedDatabaseCatalogStatus() distributeddatabasesdk.DistributedDatabaseCatalog {
	return distributeddatabasesdk.DistributedDatabaseCatalog{
		OriginalReplica: &distributeddatabasesdk.DistributedDatabaseCatalogReplicaDetails{
			Name: common.String("catalog"),
			Database: distributeddatabasesdk.DistributedDatabaseCatalogDatabaseWithExadbXsNewVaultAndCluster{
				Name:               common.String("catalog"),
				AvailabilityDomain: common.String("Uocm:PHX-AD-1"),
				Status:             distributeddatabasesdk.DistributedDatabaseCatalogDatabaseWithExadbXsNewVaultAndClusterStatusConfigured,
			},
		},
	}
}

func TestDistributedDatabaseCatalogStatusFixtureIsComparable(t *testing.T) {
	t.Parallel()
	status := distributedDatabaseCatalogStatus()
	if status.OriginalReplica == nil || status.OriginalReplica.Database == nil || !reflect.DeepEqual(status.OriginalReplica.Name, common.String("catalog")) {
		t.Fatalf("catalog status fixture = %#v", status)
	}
}
