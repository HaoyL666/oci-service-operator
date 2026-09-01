/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"testing"

	devopssdk "github.com/oracle/oci-go-sdk/v65/devops"
	loganalyticssdk "github.com/oracle/oci-go-sdk/v65/loganalytics"
	monitoringsdk "github.com/oracle/oci-go-sdk/v65/monitoring"
	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
	resourceschedulersdk "github.com/oracle/oci-go-sdk/v65/resourcescheduler"
	securityattributesdk "github.com/oracle/oci-go-sdk/v65/securityattribute"
	waassdk "github.com/oracle/oci-go-sdk/v65/waas"
	wafsdk "github.com/oracle/oci-go-sdk/v65/waf"
)

// OpenWAASSDK opens the WAAS SDK against either OCI or a checked-in cassette.
func OpenWAASSDK(t *testing.T, mode Mode, path string, metadata Metadata) (waassdk.WaasClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := waassdk.NewWaasClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://waas.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181116", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return waassdk.WaasClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenDevOpsSDK opens the DevOps SDK against either OCI or a checked-in cassette.
func OpenDevOpsSDK(t *testing.T, mode Mode, path string, metadata Metadata) (devopssdk.DevopsClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := devopssdk.NewDevopsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://devops.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210630", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return devopssdk.DevopsClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenMonitoringSDK opens the Monitoring SDK against either OCI or a checked-in cassette.
func OpenMonitoringSDK(t *testing.T, mode Mode, path string, metadata Metadata) (monitoringsdk.MonitoringClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := monitoringsdk.NewMonitoringClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://telemetry.us-ashburn-1.oraclecloud.com", BasePath: "20180401", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return monitoringsdk.MonitoringClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenNotificationSDK opens the ONS control-plane SDK against either OCI or a checked-in cassette.
func OpenNotificationSDK(t *testing.T, mode Mode, path string, metadata Metadata) (onssdk.NotificationDataPlaneClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := onssdk.NewNotificationDataPlaneClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://notification.us-ashburn-1.oraclecloud.com", BasePath: "20181201", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return onssdk.NotificationDataPlaneClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenResourceSchedulerSDK opens the Resource Scheduler SDK against either OCI or a checked-in cassette.
func OpenResourceSchedulerSDK(t *testing.T, mode Mode, path string, metadata Metadata) (resourceschedulersdk.ScheduleClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := resourceschedulersdk.NewScheduleClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://resource-scheduler.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240430", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return resourceschedulersdk.ScheduleClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenWAFSDK opens the WAF SDK against either OCI or a checked-in cassette.
func OpenWAFSDK(t *testing.T, mode Mode, path string, metadata Metadata) (wafsdk.WafClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := wafsdk.NewWafClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://waf.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210930", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return wafsdk.WafClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenLogAnalyticsSDK opens the Log Analytics SDK against either OCI or a checked-in cassette.
func OpenLogAnalyticsSDK(t *testing.T, mode Mode, path string, metadata Metadata, bindings map[string]string) (loganalyticssdk.LogAnalyticsClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loganalyticssdk.NewLogAnalyticsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://loganalytics.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200601", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return loganalyticssdk.LogAnalyticsClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenSecurityAttributeSDK opens the Security Attribute SDK against either OCI or a checked-in cassette.
func OpenSecurityAttributeSDK(t *testing.T, mode Mode, path string, metadata Metadata) (securityattributesdk.SecurityAttributeClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := securityattributesdk.NewSecurityAttributeClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://security-attribute.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240815", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return securityattributesdk.SecurityAttributeClient{BaseClient: session.BaseClient()}, session.Close
}
