/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package dbsystem

import (
	"path/filepath"
	"testing"

	mysqlsdk "github.com/oracle/oci-go-sdk/v65/mysql"
	mysqlv1beta1 "github.com/oracle/oci-service-operator/api/mysql/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDbSystemLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "dbsystem_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DbSystem OCI mock: %v", err)
		}
	})
	resource := &mysqlv1beta1.DbSystem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      syntheticDbSystemName,
			Namespace: "default",
			UID:       types.UID("synthetic-dbsystem-uid"),
		},
		Spec: mysqlv1beta1.DbSystemSpec{
			CompartmentId:        "ocid1.compartment.oc1..replay",
			ShapeName:            "MySQL.VM.Standard.E4.1.8GB",
			SubnetId:             "ocid1.subnet.oc1..replay",
			DisplayName:          syntheticDbSystemName,
			Description:          "synthetic create",
			AdminUsername:        syntheticDbSystemUsernameSource("mysql-admin"),
			AdminPassword:        syntheticDbSystemPasswordSource("mysql-admin"),
			DataStorageSizeInGBs: 50,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	ocimock.InitializeResource(resource, "mock-dbsystem")
	resource.Spec.CompartmentId = "<ocid:1>"
	resource.Spec.SubnetId = "<ocid:2>"
	sdkClient := mysqlsdk.DbSystemClient{BaseClient: session.BaseClient()}
	credentials := &syntheticDbSystemCredentialClient{
		secrets: map[string]map[string][]byte{
			"mysql-admin": {
				"username": []byte("replayadmin"),
				"password": []byte("ReplayPass123!"),
			},
		},
	}
	client := newSyntheticDbSystemClient(sdkClient, credentials)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
