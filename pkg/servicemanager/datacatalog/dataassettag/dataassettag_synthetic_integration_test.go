/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dataassettag

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

func TestSyntheticDataAssetTagCreateReadDelete(t *testing.T) {
	resource := &datacatalogv1beta1.DataAssetTag{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-dataassettag"},"spec":{"catalogId":"ocid1.datacatalog.oc1..synthetic","dataAssetKey":"asset-key","name":"osok-replay-tag"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.dataassettag.oc1..synthetic", "ACTIVE", map[string]any{"key": "tag-key"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "datacatalog", Resource: "DataAssetTag",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "dataassettag_synthetic_crud.yaml"), Host: "https://datacatalog.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190325", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datacatalogsdk.DataCatalogClient{BaseClient: session.BaseClient()}
	manager := &DataAssetTagServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDataAssetTagRuntimeHooks(manager, sdkClient)
	client := wrapDataAssetTagGeneratedClient(hooks, defaultDataAssetTagServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.DataAssetTag](buildDataAssetTagGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datacatalogv1beta1.DataAssetTag]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *datacatalogv1beta1.DataAssetTag) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datacatalogv1beta1.DataAssetTag) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created DataAssetTag status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
