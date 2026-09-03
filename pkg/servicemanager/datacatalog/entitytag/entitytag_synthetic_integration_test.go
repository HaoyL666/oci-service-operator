/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package entitytag

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datacatalogsdk "github.com/oracle/oci-go-sdk/v65/datacatalog"
	datacatalogv1beta1 "github.com/oracle/oci-service-operator/api/datacatalog/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticEntityTagCreateReadDelete(t *testing.T) {
	resource := &datacatalogv1beta1.EntityTag{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-entitytag"},"spec":{"catalogId":"ocid1.datacatalog.oc1..synthetic","dataAssetKey":"asset-key","entityKey":"entity-key","name":"osok-replay-tag"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.entitytag.oc1..synthetic", "ACTIVE", map[string]any{"key": "tag-key"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "datacatalog", Resource: "EntityTag",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "entitytag_synthetic_crud.yaml"), Host: "https://datacatalog.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190325", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datacatalogsdk.DataCatalogClient{BaseClient: session.BaseClient()}
	manager := &EntityTagServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newEntityTagRuntimeHooks(manager, sdkClient)
	client := wrapEntityTagGeneratedClient(hooks, defaultEntityTagServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.EntityTag](buildEntityTagGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datacatalogv1beta1.EntityTag]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *datacatalogv1beta1.EntityTag) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datacatalogv1beta1.EntityTag) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created EntityTag status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
