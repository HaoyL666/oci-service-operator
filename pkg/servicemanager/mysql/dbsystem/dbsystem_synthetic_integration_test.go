/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	mysqlsdk "github.com/oracle/oci-go-sdk/v65/mysql"
	mysqlv1beta1 "github.com/oracle/oci-service-operator/api/mysql/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticDbSystemName = "osok-replay-synthetic-mysql-v1"

// MySQL DB Systems provision billable compute and block storage and can take
// substantial time to create and delete. This synthetic cassette exercises the
// published SDK and generated-runtime contract without provisioning a database
// or claiming that the lifecycle was observed in a live tenancy.
func TestSyntheticDbSystemCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticDbSystemSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close DbSystem cassette: %v", err)
			}
		}
	})

	credentials := &syntheticDbSystemCredentialClient{
		secrets: map[string]map[string][]byte{
			"mysql-admin": {
				"username": []byte("replayadmin"),
				"password": []byte("ReplayPass123!"),
			},
		},
	}
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
	client := newSyntheticDbSystemClient(sdkClient, credentials)
	request := ctrl.Request{NamespacedName: types.NamespacedName{
		Name: resource.Name, Namespace: resource.Namespace,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticDbSystemConvergence(createCtx, client, resource, request); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(mysqlsdk.DbSystemLifecycleStateActive) ||
		resource.Status.DisplayName != syntheticDbSystemName ||
		resource.Status.DataStorageSizeInGBs != 50 {
		t.Fatalf("created DbSystem status = %+v", resource.Status)
	}
	if !credentials.hasEndpointSecret(resource.Name) {
		t.Fatal("active DbSystem did not create its generated endpoint Secret")
	}

	resource.Spec.DisplayName = syntheticDbSystemName + "-updated"
	resource.Spec.Description = "synthetic update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSyntheticDbSystemConvergence(ctx, client, resource, request); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated DbSystem status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	if credentials.hasEndpointSecret(resource.Name) {
		t.Fatal("deleted DbSystem retained its generated endpoint Secret")
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newSyntheticDbSystemClient(
	sdkClient mysqlsdk.DbSystemClient,
	credentials credhelper.CredentialClient,
) DbSystemServiceClient {
	manager := &DbSystemServiceManager{
		CredentialClient: credentials,
		Log: loggerutil.OSOKLogger{
			Logger: ctrl.Log.WithName("synthetic-integration"),
		},
	}
	hooks := newDbSystemDefaultRuntimeHooks(sdkClient)
	applyDbSystemRuntimeHooks(manager, &hooks)
	appendDbSystemEndpointSecretRuntimeWrapper(manager, &hooks)
	delegate := defaultDbSystemServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*mysqlv1beta1.DbSystem](
			buildDbSystemGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapDbSystemGeneratedClient(hooks, delegate)
}

func openSyntheticDbSystemSDK(t *testing.T) (mysqlsdk.DbSystemClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "mysql",
		Resource: "DbSystem",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: filepath.Join(
			"testdata",
			"recordings",
			"dbsystem_synthetic_crud.yaml",
		),
		Host:     "https://mysql.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190415",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return mysqlsdk.DbSystemClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitSyntheticDbSystemConvergence(
	ctx context.Context,
	client DbSystemServiceClient,
	resource *mysqlv1beta1.DbSystem,
	request ctrl.Request,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, request)
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("DbSystem reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

type syntheticDbSystemCredentialClient struct {
	secrets map[string]map[string][]byte
	records map[string]credhelper.SecretRecord
}

var _ credhelper.CredentialClient = (*syntheticDbSystemCredentialClient)(nil)

func (c *syntheticDbSystemCredentialClient) CreateSecret(
	_ context.Context,
	name string,
	_ string,
	labels map[string]string,
	data map[string][]byte,
) (bool, error) {
	if c.records == nil {
		c.records = map[string]credhelper.SecretRecord{}
	}
	if _, exists := c.records[name]; exists {
		return false, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, name)
	}
	c.records[name] = credhelper.SecretRecord{
		UID:    types.UID("synthetic-endpoint-secret-uid"),
		Labels: cloneSyntheticDbSystemStringMap(labels),
		Data:   cloneSyntheticDbSystemByteMap(data),
	}
	return true, nil
}

func (c *syntheticDbSystemCredentialClient) DeleteSecret(
	_ context.Context, name string, _ string,
) (bool, error) {
	if _, exists := c.records[name]; !exists {
		return false, nil
	}
	delete(c.records, name)
	return true, nil
}

func (c *syntheticDbSystemCredentialClient) GetSecret(
	_ context.Context,
	name string,
	namespace string,
) (map[string][]byte, error) {
	secret, ok := c.secrets[name]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s not found", namespace, name)
	}
	return secret, nil
}

func (c *syntheticDbSystemCredentialClient) UpdateSecret(
	_ context.Context,
	name string,
	_ string,
	labels map[string]string,
	data map[string][]byte,
) (bool, error) {
	record, exists := c.records[name]
	if !exists {
		return false, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	if labels != nil {
		record.Labels = cloneSyntheticDbSystemStringMap(labels)
	}
	record.Data = cloneSyntheticDbSystemByteMap(data)
	c.records[name] = record
	return true, nil
}

func (c *syntheticDbSystemCredentialClient) GetSecretRecord(
	_ context.Context,
	name string,
	_ string,
) (credhelper.SecretRecord, error) {
	record, exists := c.records[name]
	if !exists {
		return credhelper.SecretRecord{}, apierrors.NewNotFound(
			schema.GroupResource{Resource: "secrets"}, name,
		)
	}
	record.Labels = cloneSyntheticDbSystemStringMap(record.Labels)
	record.Data = cloneSyntheticDbSystemByteMap(record.Data)
	return record, nil
}

func (c *syntheticDbSystemCredentialClient) UpdateSecretIfCurrent(
	_ context.Context,
	name string,
	_ string,
	current credhelper.SecretRecord,
	labels map[string]string,
	data map[string][]byte,
) (bool, error) {
	record, exists := c.records[name]
	if !exists {
		return false, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	if record.UID != current.UID {
		return false, fmt.Errorf("endpoint Secret %s changed before guarded update", name)
	}
	if labels != nil {
		record.Labels = cloneSyntheticDbSystemStringMap(labels)
	}
	record.Data = cloneSyntheticDbSystemByteMap(data)
	c.records[name] = record
	return true, nil
}

func (c *syntheticDbSystemCredentialClient) DeleteSecretIfCurrent(
	_ context.Context,
	name string,
	_ string,
	current credhelper.SecretRecord,
) (bool, error) {
	record, exists := c.records[name]
	if !exists {
		return false, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	if record.UID != current.UID {
		return false, fmt.Errorf("endpoint Secret %s changed before guarded delete", name)
	}
	delete(c.records, name)
	return true, nil
}

func (c *syntheticDbSystemCredentialClient) hasEndpointSecret(name string) bool {
	_, exists := c.records[name]
	return exists
}

func cloneSyntheticDbSystemStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneSyntheticDbSystemByteMap(source map[string][]byte) map[string][]byte {
	if source == nil {
		return nil
	}
	cloned := make(map[string][]byte, len(source))
	for key, value := range source {
		cloned[key] = append([]byte(nil), value...)
	}
	return cloned
}

func syntheticDbSystemUsernameSource(name string) shared.UsernameSource {
	return shared.UsernameSource{Secret: shared.SecretSource{SecretName: name}}
}

func syntheticDbSystemPasswordSource(name string) shared.PasswordSource {
	return shared.PasswordSource{Secret: shared.SecretSource{SecretName: name}}
}
