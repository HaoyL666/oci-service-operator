/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package generatedruntime

import (
	"reflect"
	"strings"
	"testing"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
)

func TestConvertResourceManagerBitbucketCloudEmailApiTokenBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		payload    string
		targetType reflect.Type
		wantType   reflect.Type
	}{
		{
			name:       "create retains email",
			payload:    `{"configSourceProviderType":"BITBUCKET_CLOUD_ACCESS_TOKEN","apiEndpoint":"https://bitbucket.org/","email":"owner@example.com","secretId":"ocid1.vaultsecret.oc1..token"}`,
			targetType: resourceManagerConfigurationSourceCreateType,
			wantType:   reflect.TypeOf(resourcemanagersdk.CreateBitbucketCloudEmailApiTokenConfigurationSourceProviderDetails{}),
		},
		{
			name:       "update retains email",
			payload:    `{"configSourceProviderType":"BITBUCKET_CLOUD_ACCESS_TOKEN","email":"owner@example.com","displayName":"renamed"}`,
			targetType: resourceManagerConfigurationSourceUpdateType,
			wantType:   reflect.TypeOf(resourcemanagersdk.UpdateBitbucketCloudEmailApiTokenConfigurationSourceProviderDetails{}),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := convertValue(mapFromCompositeJSON(t, test.payload), test.targetType)
			if err != nil {
				t.Fatal(err)
			}
			if got := reflect.TypeOf(value.Interface()); got != test.wantType {
				t.Fatalf("resolved type = %v, want %v", got, test.wantType)
			}
		})
	}
}

func TestConvertResourceManagerBitbucketCloudRejectsLegacyDiscriminator(t *testing.T) {
	t.Parallel()

	_, err := convertValue(
		mapFromCompositeJSON(t, `{"configSourceProviderType":"BITBUCKET_CLOUD_USERNAME_APPPASSWORD"}`),
		resourceManagerConfigurationSourceCreateType,
	)
	if err == nil || !strings.Contains(err.Error(), `unsupported Resource Manager configuration source provider configSourceProviderType discriminator "BITBUCKET_CLOUD_USERNAME_APPPASSWORD"`) {
		t.Fatalf("error = %v", err)
	}
}
