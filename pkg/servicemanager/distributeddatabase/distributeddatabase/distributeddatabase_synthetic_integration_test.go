/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package distributeddatabase

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	distributeddatabasesdk "github.com/oracle/oci-go-sdk/v65/distributeddatabase"
	distributeddatabasev1beta1 "github.com/oracle/oci-service-operator/api/distributeddatabase/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDistributedDatabaseCreateReadDelete(t *testing.T) {
	resource := newTestDistributedDatabaseResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.distributeddatabase.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "distributeddatabase", Resource: "DistributedDatabase",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "distributeddatabase_synthetic_crud.yaml"), Host: "https://globaldb.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250101", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := distributeddatabasesdk.DistributedDbServiceClient{BaseClient: session.BaseClient()}
	manager := &DistributedDatabaseServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDistributedDatabaseDefaultRuntimeHooks(sdkClient)
	applyDistributedDatabaseRuntimeHooks(manager, &hooks)
	client := wrapDistributedDatabaseGeneratedClient(hooks, defaultDistributedDatabaseServiceClient{ServiceClient: generatedruntime.NewServiceClient[*distributeddatabasev1beta1.DistributedDatabase](buildDistributedDatabaseGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*distributeddatabasev1beta1.DistributedDatabase]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *distributeddatabasev1beta1.DistributedDatabase) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *distributeddatabasev1beta1.DistributedDatabase) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created DistributedDatabase status = %+v", current.Status)
			}
			return nil
		},
	})
}
