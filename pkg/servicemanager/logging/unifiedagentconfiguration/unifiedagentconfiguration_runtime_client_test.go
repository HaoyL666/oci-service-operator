package unifiedagentconfiguration

import (
	"context"
	"testing"

	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	loggingv1beta1 "github.com/oracle/oci-service-operator/api/logging/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyUnifiedAgentConfigurationRuntimeHooksBuildsCreateBodyWithMandatoryFields(t *testing.T) {
	t.Parallel()

	hooks := newUnifiedAgentConfigurationDefaultRuntimeHooks(loggingsdk.LoggingManagementClient{})
	applyUnifiedAgentConfigurationRuntimeHooks(&UnifiedAgentConfigurationServiceManager{}, &hooks)

	if hooks.BuildCreateBody == nil {
		t.Fatal("hooks.BuildCreateBody = nil, want custom create builder")
	}

	body, err := hooks.BuildCreateBody(context.Background(), newUnifiedAgentConfigurationTestResource(), "default")
	if err != nil {
		t.Fatalf("hooks.BuildCreateBody() error = %v", err)
	}

	details, ok := body.(loggingsdk.CreateUnifiedAgentConfigurationDetails)
	if !ok {
		t.Fatalf("hooks.BuildCreateBody() body type = %T, want logging.CreateUnifiedAgentConfigurationDetails", body)
	}
	if details.DisplayName == nil || *details.DisplayName != "uac-sample" {
		t.Fatalf("DisplayName = %#v, want uac-sample", details.DisplayName)
	}
	if details.Description == nil || *details.Description != "sample unified agent configuration" {
		t.Fatalf("Description = %#v, want sample unified agent configuration", details.Description)
	}

	serviceConfiguration, ok := details.ServiceConfiguration.(loggingsdk.UnifiedAgentLoggingConfiguration)
	if !ok {
		t.Fatalf("ServiceConfiguration type = %T, want logging.UnifiedAgentLoggingConfiguration", details.ServiceConfiguration)
	}
	if len(serviceConfiguration.Sources) != 1 {
		t.Fatalf("len(ServiceConfiguration.Sources) = %d, want 1", len(serviceConfiguration.Sources))
	}
	if serviceConfiguration.Destination == nil || serviceConfiguration.Destination.LogObjectId == nil || *serviceConfiguration.Destination.LogObjectId != "ocid1.log.oc1..example" {
		t.Fatalf("ServiceConfiguration.Destination.LogObjectId = %#v, want ocid1.log.oc1..example", serviceConfiguration.Destination)
	}
}

func newUnifiedAgentConfigurationTestResource() *loggingv1beta1.UnifiedAgentConfiguration {
	return &loggingv1beta1.UnifiedAgentConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "uac-sample",
			Namespace: "default",
		},
		Spec: loggingv1beta1.UnifiedAgentConfigurationSpec{
			DisplayName:   "uac-sample",
			IsEnabled:     true,
			CompartmentId: "ocid1.compartment.oc1..example",
			Description:   "sample unified agent configuration",
			ServiceConfiguration: loggingv1beta1.UnifiedAgentConfigurationServiceConfiguration{
				ConfigurationType: "LOGGING",
				Sources: []loggingv1beta1.UnifiedAgentConfigurationServiceConfigurationSource{
					{
						Name:       "app-log",
						SourceType: "LOG_TAIL",
						Paths:      []string{"/var/log/app.log"},
					},
				},
				Destination: loggingv1beta1.UnifiedAgentConfigurationServiceConfigurationDestination{
					LogObjectId: "ocid1.log.oc1..example",
				},
			},
		},
	}
}
