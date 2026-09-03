/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package suppression

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	emailsdk "github.com/oracle/oci-go-sdk/v65/email"
	emailv1beta1 "github.com/oracle/oci-service-operator/api/email/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticSuppressionCreateReadDelete(t *testing.T) {
	metadata := ocireplay.Metadata{
		Service:    "email",
		Resource:   "Suppression",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	created := `{"id":"ocid1.suppression.oc1..synthetic","compartmentId":"ocid1.tenancy.oc1..replay","emailAddress":"recipient@example.com","reason":"MANUAL"}`
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path:     filepath.Join("testdata", "recordings", "suppression_synthetic_crud.yaml"),
		Host:     "https://email.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20170907",
		Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CollectionPath:      "/20170907/suppressions",
			CreatedBody:         created,
			EmptyCollectionBody: `[]`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := emailsdk.EmailClient{BaseClient: session.BaseClient()}
	manager := &SuppressionServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newSuppressionRuntimeHooks(manager, sdkClient)
	client := wrapSuppressionGeneratedClient(hooks, defaultSuppressionServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*emailv1beta1.Suppression](
			buildSuppressionGeneratedRuntimeConfig(manager, hooks),
		),
	})
	resource := &emailv1beta1.Suppression{Spec: emailv1beta1.SuppressionSpec{
		CompartmentId: "ocid1.tenancy.oc1..replay",
		EmailAddress:  "recipient@example.com",
	}}
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*emailv1beta1.Suppression]{
		Mode:          ocireplay.ModeReplay,
		Resource:      resource,
		Client:        client,
		CloseSession:  session.Close,
		Timeout:       time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *emailv1beta1.Suppression) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *emailv1beta1.Suppression) error {
			if current.Status.EmailAddress != "recipient@example.com" {
				return fmt.Errorf("created Suppression status = %+v", current.Status)
			}
			return nil
		},
	})
}
