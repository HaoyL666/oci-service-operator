/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package rovercluster

import (
	"context"
	"fmt"
	roversdk "github.com/oracle/oci-go-sdk/v65/rover"
	roverv1beta1 "github.com/oracle/oci-service-operator/api/rover/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"reflect"
	"testing"
)

// Explicit typed service-manager lifecycle; recorded and synthetic evidence is authoring reference only.
func TestMockIntegrationRoverClusterLifecycleCRUD(t *testing.T) {
	t.Parallel()

	resource := &roverv1beta1.RoverCluster{
		Spec: roverv1beta1.RoverClusterSpec{
			DisplayName:   syntheticRoverClusterName,
			CompartmentId: "ocid1.compartment.oc1..replay",
			ClusterSize:   5,
			ClusterType:   "STANDALONE",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	ocimock.InitializeResource(resource, "mock-rovercluster")
	resource.Spec = ocimock.MustJSONFixture[roverv1beta1.RoverClusterSpec](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "\u003cocid:1\u003e",
  "displayName": "osok-replay-synthetic-rover-v1",
  "freeformTags": {
    "osok-replay": "create"
  }
}`)
	updatedSpec := resource.Spec
	ocimock.MustMergeJSONFixture(t, &updatedSpec, `{
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  }
}`)
	createRequest := ocimock.MustJSONFixture[roversdk.CreateRoverClusterDetails](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "\u003cocid:1\u003e",
  "displayName": "osok-replay-synthetic-rover-v1",
  "freeformTags": {
    "osok-replay": "create"
  }
}`)
	createdState := ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1",
  "freeformTags": {
    "osok-replay": "create"
  },
  "id": "<ocid:2>",
  "lifecycleState": "ACTIVE",
  "timeCreated": "2026-08-31T12:00:00Z"
}`)
	createdReadStates := []roversdk.RoverCluster{
		ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1",
  "freeformTags": {
    "osok-replay": "create"
  },
  "id": "<ocid:2>",
  "lifecycleState": "ACTIVE",
  "timeCreated": "2026-08-31T12:00:00Z"
}`),
	}
	updateRequest := ocimock.MustJSONFixture[roversdk.UpdateRoverClusterDetails](t, `{
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  }
}`)
	updatedState := ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  },
  "id": "<ocid:2>",
  "lifecycleState": "ACTIVE",
  "timeCreated": "2026-08-31T12:00:00Z"
}`)
	updatedReadStates := []roversdk.RoverCluster{
		ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  },
  "id": "<ocid:2>",
  "lifecycleState": "ACTIVE",
  "timeCreated": "2026-08-31T12:00:00Z"
}`),
	}
	deletedReadStates := []roversdk.RoverCluster{
		ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  },
  "id": "<ocid:2>",
  "lifecycleState": "DELETING",
  "timeCreated": "2026-08-31T12:00:00Z"
}`),
		ocimock.MustOCIResponseFixture[roversdk.RoverCluster](t, `{
  "clusterSize": 5,
  "clusterType": "STANDALONE",
  "compartmentId": "<ocid:1>",
  "displayName": "osok-replay-synthetic-rover-v1-updated",
  "freeformTags": {
    "osok-replay": "update"
  },
  "id": "<ocid:2>",
  "lifecycleState": "DELETED",
  "timeCreated": "2026-08-31T12:00:00Z"
}`),
	}
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[
		roversdk.RoverCluster,
		roversdk.CreateRoverClusterDetails,
		roversdk.UpdateRoverClusterDetails,
	]{
		CollectionPath:    "/20201210/roverClusters",
		ItemPath:          "/20201210/roverClusters/<ocid:2>",
		Operations:        []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		CreateRequest:     &createRequest,
		CreatedState:      &createdState,
		UpdateRequest:     &updateRequest,
		UpdatedState:      &updatedState,
		CreatedReadStates: createdReadStates,
		UpdatedReadStates: updatedReadStates,
		DeletedReadStates: deletedReadStates,
		RequireCreateRead: true,
		RequireUpdateRead: true,
		RequireDeleteRead: true,
		CreateStatus:      200,
		UpdateStatus:      200,
		DeleteStatus:      204,
		ValidateCreate: func(request ocimock.Request, _ roversdk.CreateRoverClusterDetails) error {
			if request.Header.Get("opc-retry-token") == "" {
				return fmt.Errorf("create retry token is empty")
			}
			return nil
		},
		ValidateDelete: func(request ocimock.Request, _ roversdk.RoverCluster) error {
			if len(request.Body) != 0 {
				return fmt.Errorf("delete body = %s", request.Body)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://oci.mock.invalid", BasePath: "20201210", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close RoverCluster OCI mock: %v", err)
		}
	})
	sdkClient := roversdk.RoverClusterClient{BaseClient: session.BaseClient()}
	manager := newSyntheticRoverClusterManager(sdkClient)
	client := manager.client
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*roverv1beta1.RoverCluster]{
		Resource:      resource,
		Client:        client,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *roverv1beta1.RoverCluster) error {
			if current.Status.Id != "<ocid:2>" ||
				string(current.Status.OsokStatus.Ocid) != "<ocid:2>" ||
				current.Status.LifecycleState != "ACTIVE" ||
				!reflect.DeepEqual(current.Status.ClusterSize, current.Spec.ClusterSize) ||
				!reflect.DeepEqual(current.Status.ClusterType, current.Spec.ClusterType) ||
				!reflect.DeepEqual(current.Status.CompartmentId, current.Spec.CompartmentId) ||
				!reflect.DeepEqual(current.Status.DisplayName, current.Spec.DisplayName) ||
				!reflect.DeepEqual(current.Status.FreeformTags, current.Spec.FreeformTags) {
				return fmt.Errorf("created RoverCluster status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *roverv1beta1.RoverCluster) { current.Spec = updatedSpec },
		ValidateUpdated: func(current *roverv1beta1.RoverCluster) error {
			if current.Status.Id != "<ocid:2>" ||
				string(current.Status.OsokStatus.Ocid) != "<ocid:2>" ||
				current.Status.LifecycleState != "ACTIVE" ||
				!reflect.DeepEqual(current.Status.DisplayName, current.Spec.DisplayName) ||
				!reflect.DeepEqual(current.Status.FreeformTags, current.Spec.FreeformTags) {
				return fmt.Errorf("updated RoverCluster status = %+v", current.Status)
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
