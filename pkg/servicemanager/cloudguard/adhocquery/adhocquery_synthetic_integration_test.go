/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package adhocquery

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAdhocQueryCreateReadDelete(t *testing.T) {
	resource := &cloudguardv1beta1.AdhocQuery{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-adhocquery"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","adhocQueryDetails":{"query":"select * from cloudguard","adhocQueryResources":[{"region":"us-ashburn-1","resourceIds":["ocid1.instance.oc1.iad.synthetic"],"resourceType":"INSTANCE"}]}}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.adhocquery.oc1..synthetic", "ACTIVE", map[string]any{"status": "COMPLETED"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "cloudguard", Resource: "AdhocQuery",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "adhocquery_synthetic_crud.yaml"), Host: "https://cloudguard-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200131", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudguardsdk.CloudGuardClient{BaseClient: session.BaseClient()}
	manager := &AdhocQueryServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAdhocQueryRuntimeHooks(manager, sdkClient)
	client := wrapAdhocQueryGeneratedClient(hooks, defaultAdhocQueryServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*cloudguardv1beta1.AdhocQuery](buildAdhocQueryGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudguardv1beta1.AdhocQuery]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *cloudguardv1beta1.AdhocQuery) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudguardv1beta1.AdhocQuery) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created AdhocQuery status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
