/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package template

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
	resourcemanagerv1beta1 "github.com/oracle/oci-service-operator/api/resourcemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTemplateName = "osok-replay-resource-manager-template-v1"

func TestRecordedTemplateCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTemplateRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedTemplateSDK(t, mode)
	resource := &resourcemanagerv1beta1.Template{Spec: resourcemanagerv1beta1.TemplateSpec{
		CompartmentId:   compartmentID,
		DisplayName:     recordedTemplateName,
		Description:     "recorded create",
		LongDescription: "A private template used only for deterministic OSOK lifecycle recording.",
		TemplateConfigSource: resourcemanagerv1beta1.TemplateConfigSource{
			TemplateConfigSourceType: "ZIP_UPLOAD",
			ZipFileBase64Encoded:     recordedTemplateZip(t),
		},
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &TemplateServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newTemplateDefaultRuntimeHooks(sdkClient)
	client := wrapTemplateGeneratedClient(hooks, defaultTemplateServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*resourcemanagerv1beta1.Template](buildTemplateGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*resourcemanagerv1beta1.Template]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 10 * time.Second, Timeout: 30 * time.Minute, CleanupTimeout: 15 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity: func(current *resourcemanagerv1beta1.Template) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *resourcemanagerv1beta1.Template) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedTemplateName {
				return fmt.Errorf("created Template status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *resourcemanagerv1beta1.Template) {
			current.Spec.DisplayName = recordedTemplateName + "-updated"
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *resourcemanagerv1beta1.Template) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Template status = %+v", current.Status)
			}
			return nil
		},
	})
}

func recordedTemplateZip(t *testing.T) string {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("terraform { required_version = \">= 1.0\" }\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func openRecordedTemplateSDK(t *testing.T, mode ocireplay.Mode) (resourcemanagersdk.ResourceManagerClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "resourcemanager", Resource: "Template",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "template_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := resourcemanagersdk.NewResourceManagerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://resourcemanager.us-ashburn-1.oraclecloud.com", BasePath: "20180917", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return resourcemanagersdk.ResourceManagerClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredTemplateRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
