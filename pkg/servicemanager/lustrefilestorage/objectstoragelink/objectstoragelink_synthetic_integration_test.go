/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package objectstoragelink

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	lustrefilestoragesdk "github.com/oracle/oci-go-sdk/v65/lustrefilestorage"
	lustrefilestoragev1beta1 "github.com/oracle/oci-service-operator/api/lustrefilestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticObjectStorageLinkCreateReadDelete(t *testing.T) {
	resource := newObjectStorageLinkTestResource()
	resourceID := "ocid1.objectstoragelink.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(observedObjectStorageLinkFromSpec(resourceID, resource.Spec, "ACTIVE"))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeObjectStorageLinkWorkRequest("ocid1.workrequest.oc1..syntheticdelete", resourceID, lustrefilestoragesdk.OperationStatusSucceeded))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "lustrefilestorage", Resource: "ObjectStorageLink", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "objectstoragelink_synthetic_crud.yaml"), Host: "https://lustre-file-storage.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250228", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20250228/objectStorageLinks", CreatedBody: createdBody, CreateStatus: 201, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := lustrefilestoragesdk.LustreFileStorageClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &ObjectStorageLinkServiceManager{Log: log}
	hooks := newObjectStorageLinkDefaultRuntimeHooks(sdkClient)
	applyObjectStorageLinkRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapObjectStorageLinkGeneratedClient(hooks, defaultObjectStorageLinkServiceClient{ServiceClient: generatedruntime.NewServiceClient[*lustrefilestoragev1beta1.ObjectStorageLink](buildObjectStorageLinkGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lustrefilestoragev1beta1.ObjectStorageLink]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *lustrefilestoragev1beta1.ObjectStorageLink) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *lustrefilestoragev1beta1.ObjectStorageLink) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ObjectStorageLink status = %+v", current.Status)
		}
		return nil
	}})
}
