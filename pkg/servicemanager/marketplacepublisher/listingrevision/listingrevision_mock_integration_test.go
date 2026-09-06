/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package listingrevision

import (
	"context"
	"encoding/json"
	"fmt"
	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	marketplacepublisherv1beta1 "github.com/oracle/oci-service-operator/api/marketplacepublisher/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
)

// Explicit typed service-manager lifecycle; recorded and synthetic evidence is authoring reference only.
func TestMockIntegrationListingRevisionLifecycleCRUD(t *testing.T) {
	t.Parallel()

	resource := makeListingRevisionResource()
	ocimock.InitializeResource(resource, "mock-listingrevision")
	resource.Spec = ocimock.MustJSONFixture[marketplacepublisherv1beta1.ListingRevisionSpec](t, `{
  "displayName": "Partner service",
  "freeformTags": {
    "env": "dev"
  },
  "headline": "Partner service headline",
  "industries": [
    "Technology"
  ],
  "listingId": "\u003cocid:1\u003e",
  "listingType": "SERVICE",
  "productCodes": [
    "COMPUTE"
  ],
  "tagline": "Initial tagline"
}`)
	createRequest := ocimock.MustJSONFixture[marketplacepublishersdk.CreateServiceListingRevisionDetails](t, `{
  "displayName": "Partner service",
  "freeformTags": {
    "env": "dev"
  },
  "headline": "Partner service headline",
  "industries": [
    "Technology"
  ],
  "listingId": "\u003cocid:1\u003e",
  "productCodes": [
    "COMPUTE"
  ],
  "tagline": "Initial tagline"
}`)
	createdState := ocimock.MustOCIResponseFixture[marketplacepublishersdk.ServiceListingRevision](t, `{
  "displayName": "Partner service",
  "freeformTags": {
    "env": "dev"
  },
  "headline": "Partner service headline",
  "id": "\u003cocid:2\u003e",
  "industries": [
    "Technology"
  ],
  "lifecycleState": "ACTIVE",
  "listingId": "\u003cocid:1\u003e",
  "listingType": "SERVICE",
  "productCodes": [
    "COMPUTE"
  ],
  "tagline": "Initial tagline"
}`)
	createdReadStates := []marketplacepublishersdk.ServiceListingRevision{
		ocimock.MustOCIResponseFixture[marketplacepublishersdk.ServiceListingRevision](t, `{
  "displayName": "Partner service",
  "freeformTags": {
    "env": "dev"
  },
  "headline": "Partner service headline",
  "id": "\u003cocid:2\u003e",
  "industries": [
    "Technology"
  ],
  "lifecycleState": "ACTIVE",
  "listingId": "\u003cocid:1\u003e",
  "listingType": "SERVICE",
  "productCodes": [
    "COMPUTE"
  ],
  "tagline": "Initial tagline"
}`),
	}
	responder, err := ocimock.NewExplicitCRUDResponder(ocimock.ExplicitCRUDOptions[
		marketplacepublishersdk.ServiceListingRevision,
		marketplacepublishersdk.CreateServiceListingRevisionDetails,
		marketplacepublishersdk.UpdateServiceListingRevisionDetails,
	]{
		CollectionPath:     "/20241201/listingRevisions",
		ItemPath:           "/20241201/listingRevisions/<ocid:2>",
		Operations:         []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationDelete},
		CreatedState:       &createdState,
		CreatedReadStates:  createdReadStates,
		DeleteEndsNotFound: true,
		RequireCreateRead:  true,
		RequireDeleteRead:  true,
		CreateStatus:       201,
		DeleteStatus:       204,
		NotFoundCode:       "NotFound",
		ValidateCreateRaw: func(request ocimock.Request) error {
			var body map[string]any
			if err := json.Unmarshal(request.Body, &body); err != nil {
				return err
			}
			if body["listingType"] != "SERVICE" {
				return fmt.Errorf("create listingType = %#v", body["listingType"])
			}
			delete(body, "listingType")
			normalized, err := json.Marshal(body)
			if err != nil {
				return err
			}
			request.Body = normalized
			if err := ocimock.ValidateJSONRequest(request, createRequest); err != nil {
				return err
			}
			if request.Header.Get("opc-retry-token") == "" {
				return fmt.Errorf("create retry token is empty")
			}
			return nil
		},
		ValidateDelete: func(request ocimock.Request, _ marketplacepublishersdk.ServiceListingRevision) error {
			if len(request.Body) != 0 {
				return fmt.Errorf("delete body = %s", request.Body)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://oci.mock.invalid", BasePath: "20241201", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ListingRevision OCI mock: %v", err)
		}
	})
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	client := newListingRevisionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*marketplacepublisherv1beta1.ListingRevision]{
		Resource:      resource,
		Client:        client,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *marketplacepublisherv1beta1.ListingRevision) error {
			if current.Status.Id != "<ocid:2>" ||
				string(current.Status.OsokStatus.Ocid) != "<ocid:2>" ||
				current.Status.LifecycleState != "ACTIVE" ||
				!reflect.DeepEqual(current.Status.DisplayName, current.Spec.DisplayName) ||
				!reflect.DeepEqual(current.Status.FreeformTags, current.Spec.FreeformTags) ||
				!reflect.DeepEqual(current.Status.Headline, current.Spec.Headline) ||
				!reflect.DeepEqual(current.Status.Industries, current.Spec.Industries) ||
				!reflect.DeepEqual(current.Status.ListingId, current.Spec.ListingId) ||
				!reflect.DeepEqual(current.Status.ListingType, current.Spec.ListingType) ||
				!reflect.DeepEqual(current.Status.ProductCodes, current.Spec.ProductCodes) ||
				!reflect.DeepEqual(current.Status.Tagline, current.Spec.Tagline) {
				return fmt.Errorf("created ListingRevision status = %+v", current.Status)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
