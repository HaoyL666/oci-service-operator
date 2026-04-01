/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	mysqlsdk "github.com/oracle/oci-go-sdk/v65/mysql"
	mysqlv1beta1 "github.com/oracle/oci-service-operator/api/mysql/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

var newMySqlDbSystemRuntimeOCIClient = func(provider common.ConfigurationProvider) (MySQLDbSystemClientInterface, error) {
	return mysqlsdk.NewDbSystemClientWithConfigurationProvider(provider)
}

func init() {
	newMySqlDbSystemServiceClient = func(manager *MySqlDbSystemServiceManager) MySqlDbSystemServiceClient {
		sdkClient, err := newMySqlDbSystemRuntimeOCIClient(manager.Provider)
		return newGeneratedMySqlDbSystemServiceClient(manager, sdkClient, err)
	}
}

type generatedMySqlDbSystemServiceClient struct {
	runtime   generatedruntime.ServiceClient[*mysqlv1beta1.MySqlDbSystem]
	ociClient MySQLDbSystemClientInterface
	initErr   error
}

var _ MySqlDbSystemServiceClient = generatedMySqlDbSystemServiceClient{}

func newGeneratedMySqlDbSystemServiceClient(
	manager *MySqlDbSystemServiceManager,
	ociClient MySQLDbSystemClientInterface,
	initErr error,
) MySqlDbSystemServiceClient {
	cfg := generatedruntime.Config[*mysqlv1beta1.MySqlDbSystem]{
		Kind:    "MySqlDbSystem",
		SDKName: "DbSystem",
		Log:     manager.Log,
		BuildCreateBody: func(ctx context.Context, resource *mysqlv1beta1.MySqlDbSystem, req ctrl.Request) (any, error) {
			return buildMySqlCreateDbSystemDetails(
				ctx,
				manager.CredentialClient,
				resource,
				requestNamespace(req, resource.Namespace),
			)
		},
		BuildUpdateBody: func(_ context.Context, resource *mysqlv1beta1.MySqlDbSystem, _ ctrl.Request) (any, error) {
			return buildMySqlUpdateDbSystemDetails(resource), nil
		},
		AfterSuccess: func(ctx context.Context, resource *mysqlv1beta1.MySqlDbSystem, _ ctrl.Request, response any) error {
			return maybeWriteMySqlEndpointSecret(ctx, manager.CredentialClient, resource, response)
		},
		Create: &generatedruntime.Operation{
			NewRequest: func() any { return &mysqlsdk.CreateDbSystemRequest{} },
			Call: func(ctx context.Context, request any) (any, error) {
				return ociClient.CreateDbSystem(ctx, *request.(*mysqlsdk.CreateDbSystemRequest))
			},
		},
		Get: &generatedruntime.Operation{
			NewRequest: func() any { return &mysqlsdk.GetDbSystemRequest{} },
			Call: func(ctx context.Context, request any) (any, error) {
				return ociClient.GetDbSystem(ctx, *request.(*mysqlsdk.GetDbSystemRequest))
			},
		},
		Update: &generatedruntime.Operation{
			NewRequest: func() any { return &mysqlsdk.UpdateDbSystemRequest{} },
			Call: func(ctx context.Context, request any) (any, error) {
				return updateMySqlDbSystemIfNeeded(ctx, ociClient, *request.(*mysqlsdk.UpdateDbSystemRequest))
			},
		},
	}
	if initErr != nil {
		cfg.InitError = fmt.Errorf("initialize MySqlDbSystem OCI client: %w", initErr)
	}

	return generatedMySqlDbSystemServiceClient{
		runtime:   generatedruntime.NewServiceClient[*mysqlv1beta1.MySqlDbSystem](cfg),
		ociClient: ociClient,
		initErr:   initErr,
	}
}

func (c generatedMySqlDbSystemServiceClient) CreateOrUpdate(
	ctx context.Context,
	resource *mysqlv1beta1.MySqlDbSystem,
	req ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if c.initErr == nil && c.ociClient != nil && currentMySqlDbSystemID(resource) == "" {
		existingID, err := findExistingMySqlDbSystemID(ctx, c.ociClient, resource)
		if err != nil {
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		if existingID != nil {
			resource.Status.OsokStatus.Ocid = *existingID
		}
	}

	return c.runtime.CreateOrUpdate(ctx, resource, req)
}

func (c generatedMySqlDbSystemServiceClient) Delete(
	ctx context.Context,
	resource *mysqlv1beta1.MySqlDbSystem,
) (bool, error) {
	if c.initErr != nil {
		return c.runtime.Delete(ctx, resource)
	}
	return true, nil
}

func requestNamespace(req ctrl.Request, fallback string) string {
	if req.Namespace != "" {
		return req.Namespace
	}
	return fallback
}

func currentMySqlDbSystemID(resource *mysqlv1beta1.MySqlDbSystem) string {
	if resource == nil {
		return ""
	}
	if resource.Status.OsokStatus.Ocid != "" {
		return string(resource.Status.OsokStatus.Ocid)
	}
	return strings.TrimSpace(string(resource.Spec.MySqlDbSystemId))
}

func findExistingMySqlDbSystemID(
	ctx context.Context,
	ociClient MySQLDbSystemClientInterface,
	resource *mysqlv1beta1.MySqlDbSystem,
) (*shared.OCID, error) {
	response, err := ociClient.ListDbSystems(ctx, mysqlsdk.ListDbSystemsRequest{
		CompartmentId: common.String(string(resource.Spec.CompartmentId)),
		DisplayName:   common.String(resource.Spec.DisplayName),
		Limit:         common.Int(1),
	})
	if err != nil {
		return nil, err
	}

	for _, item := range response.Items {
		switch item.LifecycleState {
		case mysqlsdk.DbSystemLifecycleStateActive,
			mysqlsdk.DbSystemLifecycleStateCreating,
			mysqlsdk.DbSystemLifecycleStateUpdating,
			mysqlsdk.DbSystemLifecycleStateInactive:
			ocid := shared.OCID("")
			if item.Id != nil {
				ocid = shared.OCID(*item.Id)
			}
			return &ocid, nil
		}
	}
	return nil, nil
}

func buildMySqlCreateDbSystemDetails(
	ctx context.Context,
	credClient credhelper.CredentialClient,
	resource *mysqlv1beta1.MySqlDbSystem,
	namespace string,
) (any, error) {
	adminUsername, err := readMySqlSecretValue(
		ctx,
		credClient,
		resource.Spec.AdminUsername.Secret.SecretName,
		namespace,
		"username",
		"username key in admin secret is not found",
	)
	if err != nil {
		return nil, err
	}
	adminPassword, err := readMySqlSecretValue(
		ctx,
		credClient,
		resource.Spec.AdminPassword.Secret.SecretName,
		namespace,
		"password",
		"password key in admin secret is not found",
	)
	if err != nil {
		return nil, err
	}

	details := mysqlsdk.CreateDbSystemDetails{
		ShapeName:            common.String(resource.Spec.ShapeName),
		AvailabilityDomain:   common.String(resource.Spec.AvailabilityDomain),
		FaultDomain:          common.String(resource.Spec.FaultDomain),
		IsHighlyAvailable:    common.Bool(resource.Spec.IsHighlyAvailable),
		CompartmentId:        common.String(string(resource.Spec.CompartmentId)),
		DataStorageSizeInGBs: common.Int(resource.Spec.DataStorageSizeInGBs),
		SubnetId:             common.String(string(resource.Spec.SubnetId)),
		AdminUsername:        common.String(adminUsername),
		AdminPassword:        common.String(adminPassword),
		DisplayName:          common.String(resource.Spec.DisplayName),
		DefinedTags:          *util.ConvertToOciDefinedTags(&resource.Spec.DefinedTags),
		FreeformTags:         resource.Spec.FreeFormTags,
	}
	if resource.Spec.Description != "" {
		details.Description = common.String(resource.Spec.Description)
	}
	if resource.Spec.Port != 0 {
		details.Port = common.Int(resource.Spec.Port)
	}
	if resource.Spec.PortX != 0 {
		details.PortX = common.Int(resource.Spec.PortX)
	}
	if resource.Spec.ConfigurationId.Id != "" {
		details.ConfigurationId = common.String(string(resource.Spec.ConfigurationId.Id))
	}
	if resource.Spec.IpAddress != "" {
		details.IpAddress = common.String(resource.Spec.IpAddress)
	}
	if resource.Spec.HostnameLabel != "" {
		details.HostnameLabel = common.String(resource.Spec.HostnameLabel)
	}
	if resource.Spec.MysqlVersion != "" {
		details.MysqlVersion = common.String(resource.Spec.MysqlVersion)
	}

	return details, nil
}

func readMySqlSecretValue(
	ctx context.Context,
	credClient credhelper.CredentialClient,
	secretName string,
	namespace string,
	key string,
	missingKeyMessage string,
) (string, error) {
	if credClient == nil {
		return "", fmt.Errorf("credential client is required for mysql secret resolution")
	}

	values, err := credClient.GetSecret(ctx, secretName, namespace)
	if err != nil {
		return "", err
	}
	value, ok := values[key]
	if !ok {
		return "", errors.New(missingKeyMessage)
	}
	return string(value), nil
}

func buildMySqlUpdateDbSystemDetails(resource *mysqlv1beta1.MySqlDbSystem) any {
	details := mysqlsdk.UpdateDbSystemDetails{}
	if resource.Spec.DisplayName != "" {
		details.DisplayName = common.String(resource.Spec.DisplayName)
	}
	if resource.Spec.Description != "" {
		details.Description = common.String(resource.Spec.Description)
	}
	if resource.Spec.ConfigurationId.Id != "" {
		details.ConfigurationId = common.String(string(resource.Spec.ConfigurationId.Id))
	}
	if resource.Spec.FreeFormTags != nil {
		details.FreeformTags = resource.Spec.FreeFormTags
	}
	if resource.Spec.DefinedTags != nil {
		details.DefinedTags = *util.ConvertToOciDefinedTags(&resource.Spec.DefinedTags)
	}
	return details
}

func updateMySqlDbSystemIfNeeded(
	ctx context.Context,
	ociClient MySQLDbSystemClientInterface,
	request mysqlsdk.UpdateDbSystemRequest,
) (any, error) {
	current, err := ociClient.GetDbSystem(ctx, mysqlsdk.GetDbSystemRequest{DbSystemId: request.DbSystemId})
	if err != nil {
		return nil, err
	}

	details := request.UpdateDbSystemDetails
	trimMySqlUpdateDetails(&details, current.DbSystem)
	if reflect.DeepEqual(details, mysqlsdk.UpdateDbSystemDetails{}) {
		return current, nil
	}

	request.UpdateDbSystemDetails = details
	return ociClient.UpdateDbSystem(ctx, request)
}

func trimMySqlUpdateDetails(details *mysqlsdk.UpdateDbSystemDetails, current mysqlsdk.DbSystem) {
	if details.DisplayName != nil && current.DisplayName != nil && *details.DisplayName == *current.DisplayName {
		details.DisplayName = nil
	}
	if details.Description != nil && current.Description != nil && *details.Description == *current.Description {
		details.Description = nil
	}
	if details.ConfigurationId != nil && current.ConfigurationId != nil && *details.ConfigurationId == *current.ConfigurationId {
		details.ConfigurationId = nil
	}
	if details.FreeformTags != nil && reflect.DeepEqual(details.FreeformTags, current.FreeformTags) {
		details.FreeformTags = nil
	}
	if details.DefinedTags != nil && reflect.DeepEqual(details.DefinedTags, current.DefinedTags) {
		details.DefinedTags = nil
	}
}

func maybeWriteMySqlEndpointSecret(
	ctx context.Context,
	credClient credhelper.CredentialClient,
	resource *mysqlv1beta1.MySqlDbSystem,
	response any,
) error {
	dbSystem, ok := mysqlDbSystemFromResponse(response)
	if !ok || dbSystem.LifecycleState != mysqlsdk.DbSystemLifecycleStateActive {
		return nil
	}
	if credClient == nil {
		return fmt.Errorf("credential client is required for mysql endpoint materialization")
	}

	credMap, err := getCredentialMap(*dbSystem)
	if err != nil {
		return err
	}
	_, err = credClient.CreateSecret(ctx, resource.Name, resource.Namespace, nil, credMap)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func mysqlDbSystemFromResponse(response any) (*mysqlsdk.DbSystem, bool) {
	switch typed := response.(type) {
	case mysqlsdk.GetDbSystemResponse:
		return &typed.DbSystem, true
	case *mysqlsdk.GetDbSystemResponse:
		if typed == nil {
			return nil, false
		}
		return &typed.DbSystem, true
	case mysqlsdk.CreateDbSystemResponse:
		return &typed.DbSystem, true
	case *mysqlsdk.CreateDbSystemResponse:
		if typed == nil {
			return nil, false
		}
		return &typed.DbSystem, true
	case mysqlsdk.DbSystem:
		return &typed, true
	case *mysqlsdk.DbSystem:
		if typed == nil {
			return nil, false
		}
		return typed, true
	default:
		return nil, false
	}
}
