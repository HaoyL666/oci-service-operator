/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"testing"

	loadbalancersdk "github.com/oracle/oci-go-sdk/v65/loadbalancer"
	networkloadbalancersdk "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

// OpenLoadBalancerSDK opens the classic Load Balancer SDK against either OCI
// or a checked-in sanitized cassette.
func OpenLoadBalancerSDK(t *testing.T, mode Mode, path string, metadata Metadata) (loadbalancersdk.LoadBalancerClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loadbalancersdk.NewLoadBalancerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://iaas.us-ashburn-1.oraclecloud.com", BasePath: "20170115", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return loadbalancersdk.LoadBalancerClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenNetworkLoadBalancerSDK opens the Network Load Balancer SDK against OCI
// or a checked-in sanitized cassette.
func OpenNetworkLoadBalancerSDK(t *testing.T, mode Mode, path string, metadata Metadata) (networkloadbalancersdk.NetworkLoadBalancerClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := networkloadbalancersdk.NewNetworkLoadBalancerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://network-load-balancer-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200501", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return networkloadbalancersdk.NetworkLoadBalancerClient{BaseClient: session.BaseClient()}, session.Close
}
