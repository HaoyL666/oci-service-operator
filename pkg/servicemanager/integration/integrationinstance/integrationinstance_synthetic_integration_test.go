/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package integrationinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	integrationsdk "github.com/oracle/oci-go-sdk/v65/integration"
	integrationv1beta1 "github.com/oracle/oci-service-operator/api/integration/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// An Integration instance allocates paid managed integration capacity and identity resources.
func TestSyntheticIntegrationInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "integration", Resource: "IntegrationInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "integrationinstance_synthetic_crud.yaml"), Host: "https://integration.us-ashburn-1.ocp.oraclecloud.com", BasePath: "20190131", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := integrationsdk.IntegrationInstanceClient{BaseClient: session.BaseClient()}
	hooks := newIntegrationInstanceDefaultRuntimeHooks(sdkClient)
	applyIntegrationInstanceRuntimeHooks(&hooks)
	client := newIntegrationInstanceRuntimeTestClient(hooks)
	resource := newIntegrationInstanceTestResource()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*integrationv1beta1.IntegrationInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *integrationv1beta1.IntegrationInstance) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *integrationv1beta1.IntegrationInstance) error {
		if current.Status.DisplayName != "integration-sample" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created IntegrationInstance status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *integrationv1beta1.IntegrationInstance) {
		current.Spec.DisplayName = "integration-updated"
	}, ValidateUpdated: func(current *integrationv1beta1.IntegrationInstance) error {
		if current.Status.DisplayName != "integration-updated" {
			return fmt.Errorf("updated IntegrationInstance status = %+v", current.Status)
		}
		return nil
	}})
}
