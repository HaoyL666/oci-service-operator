/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package authenticationprovider

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAuthenticationProviderCreateReadDelete(t *testing.T) {
	resource := makeAuthenticationProviderResource()
	createdBody, err := json.Marshal(makeSDKAuthenticationProvider(testAuthenticationProviderID, resource, odasdk.LifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:    "oda",
		Resource:   "AuthenticationProvider",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path:     filepath.Join("testdata", "recordings", "authenticationprovider_synthetic_crud.yaml"),
		Host:     "https://digitalassistant-api.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190506",
		Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CollectionPath:      "/20190506/odaInstances/" + testOdaInstanceID + "/authenticationProviders",
			CreatedBody:         string(createdBody),
			EmptyCollectionBody: `{"items":[]}`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newAuthenticationProviderServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.AuthenticationProvider]{
		Mode:         ocireplay.ModeReplay,
		Resource:     resource,
		Client:       client,
		CloseSession: session.Close,
		Timeout:      time.Minute,
		HasIdentity:  func(current *odav1beta1.AuthenticationProvider) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *odav1beta1.AuthenticationProvider) error {
			if current.Status.Name != resource.Spec.Name {
				return fmt.Errorf("created AuthenticationProvider status = %+v", current.Status)
			}
			return nil
		},
	})
}
