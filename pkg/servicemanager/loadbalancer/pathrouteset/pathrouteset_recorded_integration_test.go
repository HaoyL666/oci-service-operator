/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package pathrouteset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lbv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedPathRouteSetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "PathRouteSet", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	loadBalancerID, backendSetName := "ocid1.loadbalancer.oc1..replay", "osok_replay_backend_set"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredPathRouteSetRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
		backendSetName = requiredPathRouteSetRecordingEnv(t, "OCI_REPLAY_LB_BACKEND_SET_NAME")
	}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "pathrouteset_crud.yaml"), metadata)
	resource := &lbv1beta1.PathRouteSet{Spec: lbv1beta1.PathRouteSetSpec{Name: "osok_replay_path_routes", LoadBalancerId: loadBalancerID, PathRoutes: []lbv1beta1.PathRouteSetPathRoute{pathRouteSetPathRoute("/images", backendSetName, "PREFIX_MATCH")}}}
	hooks := newPathRouteSetRuntimeHooksWithOCIClient(sdkClient)
	applyPathRouteSetRuntimeHooks(&hooks)
	manager := &PathRouteSetServiceManager{}
	client := wrapPathRouteSetGeneratedClient(hooks, defaultPathRouteSetServiceClient{ServiceClient: generatedruntime.NewServiceClient[*lbv1beta1.PathRouteSet](buildPathRouteSetGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lbv1beta1.PathRouteSet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "generated runtime resource not found") },
		HasIdentity:   func(current *lbv1beta1.PathRouteSet) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *lbv1beta1.PathRouteSet) error {
			if current.Status.Name != resource.Spec.Name || len(current.Status.PathRoutes) != 1 {
				return fmt.Errorf("created PathRouteSet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *lbv1beta1.PathRouteSet) { current.Spec.PathRoutes[0].Path = "/assets" },
		ValidateUpdated: func(current *lbv1beta1.PathRouteSet) error {
			if len(current.Status.PathRoutes) != 1 || current.Status.PathRoutes[0].Path != "/assets" {
				return fmt.Errorf("updated PathRouteSet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredPathRouteSetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
