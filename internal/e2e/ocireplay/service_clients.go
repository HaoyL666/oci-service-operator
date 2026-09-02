/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"testing"

	announcementsservicesdk "github.com/oracle/oci-go-sdk/v65/announcementsservice"
	apmconfigsdk "github.com/oracle/oci-go-sdk/v65/apmconfig"
	apmsyntheticssdk "github.com/oracle/oci-go-sdk/v65/apmsynthetics"
	apmtracessdk "github.com/oracle/oci-go-sdk/v65/apmtraces"
	artifactssdk "github.com/oracle/oci-go-sdk/v65/artifacts"
	bastionsdk "github.com/oracle/oci-go-sdk/v65/bastion"
	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	dataflowsdk "github.com/oracle/oci-go-sdk/v65/dataflow"
	devopssdk "github.com/oracle/oci-go-sdk/v65/devops"
	dnssdk "github.com/oracle/oci-go-sdk/v65/dns"
	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	genericartifactscontentsdk "github.com/oracle/oci-go-sdk/v65/genericartifactscontent"
	iotsdk "github.com/oracle/oci-go-sdk/v65/iot"
	jmssdk "github.com/oracle/oci-go-sdk/v65/jms"
	loganalyticssdk "github.com/oracle/oci-go-sdk/v65/loganalytics"
	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	managementdashboardsdk "github.com/oracle/oci-go-sdk/v65/managementdashboard"
	monitoringsdk "github.com/oracle/oci-go-sdk/v65/monitoring"
	networkfirewallsdk "github.com/oracle/oci-go-sdk/v65/networkfirewall"
	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	recoverysdk "github.com/oracle/oci-go-sdk/v65/recovery"
	resourceschedulersdk "github.com/oracle/oci-go-sdk/v65/resourcescheduler"
	securityattributesdk "github.com/oracle/oci-go-sdk/v65/securityattribute"
	stackmonitoringsdk "github.com/oracle/oci-go-sdk/v65/stackmonitoring"
	usageapisdk "github.com/oracle/oci-go-sdk/v65/usageapi"
	vulnerabilityscanningsdk "github.com/oracle/oci-go-sdk/v65/vulnerabilityscanning"
	waassdk "github.com/oracle/oci-go-sdk/v65/waas"
	wafsdk "github.com/oracle/oci-go-sdk/v65/waf"
)

// OpenAnnouncementSubscriptionSDK opens the Announcements subscription SDK against OCI or replay.
func OpenAnnouncementSubscriptionSDK(t *testing.T, mode Mode, path string, metadata Metadata) (announcementsservicesdk.AnnouncementSubscriptionClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := announcementsservicesdk.NewAnnouncementSubscriptionClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://announcements.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180904", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return announcementsservicesdk.AnnouncementSubscriptionClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenAPMConfigSDK opens the APM Configuration SDK against OCI or replay.
func OpenAPMConfigSDK(t *testing.T, mode Mode, path string, metadata Metadata) (apmconfigsdk.ConfigClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := apmconfigsdk.NewConfigClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://apm-config.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210201", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return apmconfigsdk.ConfigClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenAPMSyntheticsSDK opens the APM Synthetics SDK against OCI or replay.
func OpenAPMSyntheticsSDK(t *testing.T, mode Mode, path string, metadata Metadata) (apmsyntheticssdk.ApmSyntheticClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := apmsyntheticssdk.NewApmSyntheticClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://apm-synthetic.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return apmsyntheticssdk.ApmSyntheticClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenAPMTracesSDK opens the APM Trace Explorer SDK against OCI or replay.
func OpenAPMTracesSDK(t *testing.T, mode Mode, path string, metadata Metadata) (apmtracessdk.ScheduledQueryClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := apmtracessdk.NewScheduledQueryClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://apm-trace.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return apmtracessdk.ScheduledQueryClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenBastionSDK opens the Bastion SDK against OCI or replay.
func OpenBastionSDK(t *testing.T, mode Mode, path string, metadata Metadata) (bastionsdk.BastionClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := bastionsdk.NewBastionClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://bastion.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210331", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return bastionsdk.BastionClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenCloudGuardSDK opens the Cloud Guard SDK against OCI or replay.
func OpenCloudGuardSDK(t *testing.T, mode Mode, path string, metadata Metadata) (cloudguardsdk.CloudGuardClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := cloudguardsdk.NewCloudGuardClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://cloudguard-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200131", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return cloudguardsdk.CloudGuardClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenDataFlowSDK opens the Data Flow SDK against OCI or replay.
func OpenDataFlowSDK(t *testing.T, mode Mode, path string, metadata Metadata) (dataflowsdk.DataFlowClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := dataflowsdk.NewDataFlowClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://dataflow.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200129", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return dataflowsdk.DataFlowClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenGenericArtifactsContentSDK opens the Generic Artifacts content SDK against OCI or replay.
func OpenGenericArtifactsContentSDK(t *testing.T, mode Mode, path string, metadata Metadata) (genericartifactscontentsdk.GenericArtifactsContentClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := genericartifactscontentsdk.NewGenericArtifactsContentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://generic.artifacts.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return genericartifactscontentsdk.GenericArtifactsContentClient{BaseClient: session.BaseClient()}, session.Close
}

// GenericArtifactContentByPathClients combines the content and metadata clients used by path-addressed artifact uploads.
type GenericArtifactContentByPathClients struct {
	Content   genericartifactscontentsdk.GenericArtifactsContentClient
	Artifacts artifactssdk.ArtifactsClient
}

// OpenGenericArtifactContentByPathSDK opens both Generic Artifacts clients against one OCI recording or replay cassette.
func OpenGenericArtifactContentByPathSDK(t *testing.T, mode Mode, path string, metadata Metadata, bindings map[string]string) (GenericArtifactContentByPathClients, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		contentClient, err := genericartifactscontentsdk.NewGenericArtifactsContentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		artifactsClient, err := artifactssdk.NewArtifactsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &contentClient.BaseClient, Bindings: bindings, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&artifactsClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return GenericArtifactContentByPathClients{Content: contentClient, Artifacts: artifactsClient}, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://generic.artifacts.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	contentBaseClient := session.BaseClient()
	artifactsBaseClient, err := session.BaseClientFor("https://artifacts.us-ashburn-1.oci.oraclecloud.com", "20160918")
	if err != nil {
		t.Fatal(err)
	}
	return GenericArtifactContentByPathClients{
		Content:   genericartifactscontentsdk.GenericArtifactsContentClient{BaseClient: contentBaseClient},
		Artifacts: artifactssdk.ArtifactsClient{BaseClient: artifactsBaseClient},
	}, session.Close
}

// OpenIoTSDK opens the IoT SDK against OCI or replay.
func OpenIoTSDK(t *testing.T, mode Mode, path string, metadata Metadata) (iotsdk.IotClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := iotsdk.NewIotClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://iot.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250531", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return iotsdk.IotClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenJMSSDK opens the Java Management Service SDK against OCI or replay.
func OpenJMSSDK(t *testing.T, mode Mode, path string, metadata Metadata) (jmssdk.JavaManagementServiceClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := jmssdk.NewJavaManagementServiceClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://javamanagement.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210610", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return jmssdk.JavaManagementServiceClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenManagementDashboardSDK opens the Management Dashboard SDK against OCI or replay.
func OpenManagementDashboardSDK(t *testing.T, mode Mode, path string, metadata Metadata) (managementdashboardsdk.DashxApisClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := managementdashboardsdk.NewDashxApisClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://managementdashboard.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return managementdashboardsdk.DashxApisClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenStackMonitoringSDK opens the Stack Monitoring SDK against OCI or replay.
func OpenStackMonitoringSDK(t *testing.T, mode Mode, path string, metadata Metadata) (stackmonitoringsdk.StackMonitoringClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := stackmonitoringsdk.NewStackMonitoringClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://stack-monitoring.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenRecoverySDK opens the Database Recovery SDK against OCI or replay.
func OpenRecoverySDK(t *testing.T, mode Mode, path string, metadata Metadata) (recoverysdk.DatabaseRecoveryClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := recoverysdk.NewDatabaseRecoveryClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210216", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return recoverysdk.DatabaseRecoveryClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenDNSSDK opens the DNS SDK against OCI or replay.
func OpenDNSSDK(t *testing.T, mode Mode, path string, metadata Metadata) (dnssdk.DnsClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := dnssdk.NewDnsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://dns.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180115", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return dnssdk.DnsClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenNetworkFirewallSDK opens the Network Firewall SDK against OCI or replay.
func OpenNetworkFirewallSDK(t *testing.T, mode Mode, path string, metadata Metadata, bindingOptions ...map[string]string) (networkfirewallsdk.NetworkFirewallClient, func() error) {
	t.Helper()
	var bindings map[string]string
	if len(bindingOptions) > 0 {
		bindings = bindingOptions[0]
	}
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := networkfirewallsdk.NewNetworkFirewallClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://network-firewall.us-ashburn-1.ocs.oraclecloud.com", BasePath: "20230501", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return networkfirewallsdk.NetworkFirewallClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenWAASSDK opens the WAAS SDK against either OCI or a checked-in cassette.
func OpenWAASSDK(t *testing.T, mode Mode, path string, metadata Metadata, bindingOptions ...map[string]string) (waassdk.WaasClient, func() error) {
	t.Helper()
	var bindings map[string]string
	if len(bindingOptions) > 0 {
		bindings = bindingOptions[0]
	}
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := waassdk.NewWaasClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://waas.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181116", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return waassdk.WaasClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenFileStorageSDK opens the File Storage SDK against either OCI or a checked-in cassette.
func OpenFileStorageSDK(t *testing.T, mode Mode, path string, metadata Metadata) (filestoragesdk.FileStorageClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := filestoragesdk.NewFileStorageClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://filestorage.us-ashburn-1.oci.oraclecloud.com", BasePath: "20171215", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenLoggingManagementSDK opens the Logging Management SDK against either OCI or a checked-in cassette.
func OpenLoggingManagementSDK(t *testing.T, mode Mode, path string, metadata Metadata) (loggingsdk.LoggingManagementClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loggingsdk.NewLoggingManagementClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://logging.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200531", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return loggingsdk.LoggingManagementClient{BaseClient: session.BaseClient()}, session.Close
}

// WAASRedirectClients combines the Redirect resource client with the WAAS work-request client.
type WAASRedirectClients struct {
	waassdk.RedirectClient
	waassdk.WaasClient
}

// OpenWAASRedirectSDK opens the WAAS HTTP Redirect and work-request SDKs against either OCI or a checked-in cassette.
func OpenWAASRedirectSDK(t *testing.T, mode Mode, path string, metadata Metadata) (WAASRedirectClients, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		redirectClient, err := waassdk.NewRedirectClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		workRequestClient, err := waassdk.NewWaasClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &redirectClient.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&workRequestClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return WAASRedirectClients{RedirectClient: redirectClient, WaasClient: workRequestClient}, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://waas.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181116", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	return WAASRedirectClients{
		RedirectClient: waassdk.RedirectClient{BaseClient: baseClient},
		WaasClient:     waassdk.WaasClient{BaseClient: baseClient},
	}, session.Close
}

// OpenUsageAPISDK opens the Usage API SDK against either OCI or a checked-in cassette.
func OpenUsageAPISDK(t *testing.T, mode Mode, path string, metadata Metadata, bindingOptions ...map[string]string) (usageapisdk.UsageapiClient, func() error) {
	t.Helper()
	var bindings map[string]string
	if len(bindingOptions) > 0 {
		bindings = bindingOptions[0]
	}
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := usageapisdk.NewUsageapiClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://usageapi.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200107", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return usageapisdk.UsageapiClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenOSManagementHubLifecycleEnvironmentSDK opens the OS Management Hub lifecycle-environment SDK against OCI or replay.
func OpenOSManagementHubLifecycleEnvironmentSDK(t *testing.T, mode Mode, path string, metadata Metadata) (osmanagementhubsdk.LifecycleEnvironmentClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := osmanagementhubsdk.NewLifecycleEnvironmentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return osmanagementhubsdk.LifecycleEnvironmentClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenOSManagementHubManagedInstanceGroupSDK opens the OS Management Hub managed-instance-group SDK against OCI or replay.
func OpenOSManagementHubManagedInstanceGroupSDK(t *testing.T, mode Mode, path string, metadata Metadata) (osmanagementhubsdk.ManagedInstanceGroupClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := osmanagementhubsdk.NewManagedInstanceGroupClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return osmanagementhubsdk.ManagedInstanceGroupClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenOSManagementHubProfileSDK opens the OS Management Hub profile SDK against OCI or replay.
func OpenOSManagementHubProfileSDK(t *testing.T, mode Mode, path string, metadata Metadata) (osmanagementhubsdk.OnboardingClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := osmanagementhubsdk.NewOnboardingClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return osmanagementhubsdk.OnboardingClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenOSManagementHubScheduledJobSDK opens the OS Management Hub scheduled-job SDK against OCI or replay.
func OpenOSManagementHubScheduledJobSDK(t *testing.T, mode Mode, path string, metadata Metadata) (osmanagementhubsdk.ScheduledJobClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := osmanagementhubsdk.NewScheduledJobClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return osmanagementhubsdk.ScheduledJobClient{BaseClient: session.BaseClient()}, session.Close
}

// OpenVulnerabilityScanningSDK opens the Vulnerability Scanning SDK against either OCI or a checked-in cassette.
func OpenVulnerabilityScanningSDK(t *testing.T, mode Mode, path string, metadata Metadata) (vulnerabilityscanningsdk.VulnerabilityScanningClient, func() error) {
	t.Helper()
	if mode == ModeRecord {
		provider, err := RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := vulnerabilityscanningsdk.NewVulnerabilityScanningClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := OpenSDKRecord(SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := OpenSDKReplay(SDKReplayOptions{Path: path, Host: "https://vss-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210215", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return vulnerabilityscanningsdk.VulnerabilityScanningClient{BaseClient: session.BaseClient()}, session.Close
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
