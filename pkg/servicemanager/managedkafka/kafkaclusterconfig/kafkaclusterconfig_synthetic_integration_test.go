/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package kafkaclusterconfig

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	managedkafkasdk "github.com/oracle/oci-go-sdk/v65/managedkafka"
	managedkafkav1beta1 "github.com/oracle/oci-service-operator/api/managedkafka/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticKafkaClusterConfigCreateReadDelete(t *testing.T) {
	resource := makeKafkaClusterConfigResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.kafkaclusterconfig.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "managedkafka", Resource: "KafkaClusterConfig", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "kafkaclusterconfig_synthetic_crud.yaml"), Host: "https://managed-kafka.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240901", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := managedkafkasdk.KafkaClusterClient{BaseClient: session.BaseClient()}
	client := newKafkaClusterConfigServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managedkafkav1beta1.KafkaClusterConfig]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *managedkafkav1beta1.KafkaClusterConfig) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *managedkafkav1beta1.KafkaClusterConfig) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created KafkaClusterConfig status = %+v", current.Status)
		}
		return nil
	}})
}
