/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cluster

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	ocvpsdk "github.com/oracle/oci-go-sdk/v65/ocvp"
	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticClusterCreateReadDelete(t *testing.T) {
	resource := newClusterTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.cluster.oc1..synthetic", "ACTIVE", map[string]any{"compartmentId": "ocid1.compartment.oc1..replay"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "ocvp", Resource: "Cluster", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "cluster_synthetic_crud.yaml"), Host: "https://ocvp.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230701", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := ocvpsdk.ClusterClient{BaseClient: session.BaseClient()}
	manager := &ClusterServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newClusterRuntimeHooks(manager, sdkClient)
	client := wrapClusterGeneratedClient(hooks, defaultClusterServiceClient{ServiceClient: generatedruntime.NewServiceClient[*ocvpv1beta1.Cluster](buildClusterGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*ocvpv1beta1.Cluster]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *ocvpv1beta1.Cluster) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *ocvpv1beta1.Cluster) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Cluster status = %+v", current.Status)
		}
		return nil
	}})
}
