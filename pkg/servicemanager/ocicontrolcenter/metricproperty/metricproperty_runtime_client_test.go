package metricproperty

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	ocicontrolcenterv1beta1 "github.com/oracle/oci-service-operator/api/ocicontrolcenter/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testMetricPropertyCompartmentID = "ocid1.compartment.oc1..metricproperty"
	testMetricPropertyNamespaceName = "oci_computeagent"
	testMetricPropertyMetricName    = "CpuUtilization"
)

type fakeMetricPropertyOCIClient struct {
	listFn func(context.Context, ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error)
}

func (f *fakeMetricPropertyOCIClient) ListMetricProperties(
	ctx context.Context,
	req ocicontrolcentersdk.ListMetricPropertiesRequest,
) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return ocicontrolcentersdk.ListMetricPropertiesResponse{}, nil
}

func TestReviewedMetricPropertyRuntimeSemanticsDocumentsReadOnlyListSurface(t *testing.T) {
	t.Parallel()

	hooks := &MetricPropertyRuntimeHooks{}
	applyMetricPropertyRuntimeHooks(hooks)

	got := hooks.Semantics
	if got == nil {
		t.Fatal("Semantics = nil")
	}
	if got.FormalService != "ocicontrolcenter" || got.FormalSlug != "metricproperty" {
		t.Fatalf("formal identity = %s/%s, want ocicontrolcenter/metricproperty", got.FormalService, got.FormalSlug)
	}
	if got.FinalizerPolicy != "retain-until-confirmed-delete" {
		t.Fatalf("FinalizerPolicy = %q, want retain-until-confirmed-delete", got.FinalizerPolicy)
	}
	if got.List == nil || !slices.Equal(got.List.MatchFields, []string{"metricName"}) {
		t.Fatalf("List semantics = %#v, want metricName matching", got.List)
	}
	if len(got.Mutation.Mutable) != 0 {
		t.Fatalf("Mutable fields = %#v, want none for read-only MetricProperty", got.Mutation.Mutable)
	}
}

func TestMetricPropertyCreateOrUpdateBindsListItemAndProjectsStatus(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	var gotRequest ocicontrolcentersdk.ListMetricPropertiesRequest
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(_ context.Context, req ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			gotRequest = req
			return ocicontrolcentersdk.ListMetricPropertiesResponse{
				MetricPropertyCollection: ocicontrolcentersdk.MetricPropertyCollection{
					Items: []ocicontrolcentersdk.MetricPropertySummary{
						metricPropertySummary("MemoryUtilization", nil),
						metricPropertySummary(testMetricPropertyMetricName, map[string]ocicontrolcentersdk.DimensionValue{
							"resourceId": {DimensionValue: common.String("ocid1.instance.oc1..example")},
						}),
					},
				},
				OpcRequestId: common.String("opc-list-1"),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want success", response)
	}
	requireStringPtr(t, "NamespaceName", gotRequest.NamespaceName, testMetricPropertyNamespaceName)
	requireStringPtr(t, "CompartmentId", gotRequest.CompartmentId, testMetricPropertyCompartmentID)
	if gotRequest.Page != nil {
		t.Fatalf("Page = %#v, want nil on first request", gotRequest.Page)
	}
	if resource.Status.MetricName != testMetricPropertyMetricName {
		t.Fatalf("Status.MetricName = %q, want %q", resource.Status.MetricName, testMetricPropertyMetricName)
	}
	if got := resource.Status.Dimensions["resourceId"].DimensionValue; got != "ocid1.instance.oc1..example" {
		t.Fatalf("Status.Dimensions[resourceId].DimensionValue = %q, want OCI dimension value", got)
	}
	if got := string(resource.Status.OsokStatus.Ocid); !strings.HasPrefix(got, metricPropertySyntheticIDPrefix+metricPropertySyntheticIDVersion) {
		t.Fatalf("Status.Ocid = %q, want MetricProperty synthetic identity", got)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-list-1" {
		t.Fatalf("Status.OpcRequestID = %q, want opc-list-1", resource.Status.OsokStatus.OpcRequestID)
	}
	requireCondition(t, resource.Status.OsokStatus, shared.Active, corev1.ConditionTrue)
}

func TestMetricPropertyCreateOrUpdateUsesMetadataNameWhenMetricAnnotationOmitted(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	delete(resource.Annotations, metricPropertyMetricNameAnnotation)
	resource.Name = testMetricPropertyMetricName

	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(_ context.Context, _ ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			return ocicontrolcentersdk.ListMetricPropertiesResponse{
				MetricPropertyCollection: ocicontrolcentersdk.MetricPropertyCollection{
					Items: []ocicontrolcentersdk.MetricPropertySummary{
						metricPropertySummary(testMetricPropertyMetricName, nil),
					},
				},
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want success", response)
	}
	if resource.Status.MetricName != testMetricPropertyMetricName {
		t.Fatalf("Status.MetricName = %q, want metadata name fallback", resource.Status.MetricName)
	}
}

func TestMetricPropertyCreateOrUpdateScansAllListPages(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	var pages []string
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(_ context.Context, req ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			page := ""
			if req.Page != nil {
				page = *req.Page
			}
			pages = append(pages, page)
			if page == "" {
				return ocicontrolcentersdk.ListMetricPropertiesResponse{
					MetricPropertyCollection: ocicontrolcentersdk.MetricPropertyCollection{
						Items: []ocicontrolcentersdk.MetricPropertySummary{metricPropertySummary("OtherMetric", nil)},
					},
					OpcNextPage: common.String("page-2"),
				}, nil
			}
			return ocicontrolcentersdk.ListMetricPropertiesResponse{
				MetricPropertyCollection: ocicontrolcentersdk.MetricPropertyCollection{
					Items: []ocicontrolcentersdk.MetricPropertySummary{metricPropertySummary(testMetricPropertyMetricName, nil)},
				},
				OpcRequestId: common.String("opc-page-2"),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want success", response)
	}
	if !slices.Equal(pages, []string{"", "page-2"}) {
		t.Fatalf("pages = %#v, want first page then page-2", pages)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-page-2" {
		t.Fatalf("Status.OpcRequestID = %q, want last page request id", resource.Status.OsokStatus.OpcRequestID)
	}
}

func TestMetricPropertyCreateOrUpdateRejectsSelectorDriftBeforeOCI(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	oldSelector := metricPropertySelector{
		compartmentID: "ocid1.compartment.oc1..old",
		namespaceName: testMetricPropertyNamespaceName,
		metricName:    testMetricPropertyMetricName,
	}
	resource.Status.OsokStatus.Ocid = oldSelector.syntheticOCID()
	listCalls := 0
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(context.Context, ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			listCalls++
			return ocicontrolcentersdk.ListMetricPropertiesResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "selector annotations are immutable") {
		t.Fatalf("CreateOrUpdate() error = %v, want selector drift rejection", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if listCalls != 0 {
		t.Fatalf("ListMetricProperties calls = %d, want 0 before OCI drift rejection", listCalls)
	}
	requireCondition(t, resource.Status.OsokStatus, shared.Failed, corev1.ConditionFalse)
}

func TestMetricPropertyCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(context.Context, ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			return ocicontrolcentersdk.ListMetricPropertiesResponse{}, errortest.NewServiceError(500, "InternalError", "service failed")
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "service failed") {
		t.Fatalf("CreateOrUpdate() error = %v, want OCI service error", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-request-id" {
		t.Fatalf("Status.OpcRequestID = %q, want OCI error request id", resource.Status.OsokStatus.OpcRequestID)
	}
	requireCondition(t, resource.Status.OsokStatus, shared.Failed, corev1.ConditionFalse)
}

func TestMetricPropertyCreateOrUpdateRequiresSelectorAnnotations(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	delete(resource.Annotations, metricPropertyCompartmentIDAnnotation)
	listCalls := 0
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{
		listFn: func(context.Context, ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
			listCalls++
			return ocicontrolcentersdk.ListMetricPropertiesResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), metricPropertyCompartmentIDAnnotation) {
		t.Fatalf("CreateOrUpdate() error = %v, want missing compartment annotation", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if listCalls != 0 {
		t.Fatalf("ListMetricProperties calls = %d, want 0 when selector is incomplete", listCalls)
	}
	requireCondition(t, resource.Status.OsokStatus, shared.Failed, corev1.ConditionFalse)
}

func TestMetricPropertyDeleteIsReadOnlyAndReleasesFinalizer(t *testing.T) {
	t.Parallel()

	resource := newMetricPropertyTestResource()
	resource.Status.OsokStatus.Ocid = metricPropertySelector{
		compartmentID: testMetricPropertyCompartmentID,
		namespaceName: testMetricPropertyNamespaceName,
		metricName:    testMetricPropertyMetricName,
	}.syntheticOCID()
	client := newTestMetricPropertyRuntimeClient(&fakeMetricPropertyOCIClient{})

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want true for read-only binding")
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("Status.DeletedAt = nil, want read-only delete marker")
	}
	requireCondition(t, resource.Status.OsokStatus, shared.Terminating, corev1.ConditionTrue)
}

func newTestMetricPropertyRuntimeClient(client metricPropertyOCIClient) *metricPropertyRuntimeClient {
	return newMetricPropertyRuntimeClient(
		&MetricPropertyServiceManager{Log: loggerutil.OSOKLogger{}},
		client,
		nil,
	)
}

func newMetricPropertyTestResource() *ocicontrolcenterv1beta1.MetricProperty {
	return &ocicontrolcenterv1beta1.MetricProperty{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metricproperty-sample",
			Namespace: "default",
			Annotations: map[string]string{
				metricPropertyCompartmentIDAnnotation: testMetricPropertyCompartmentID,
				metricPropertyNamespaceNameAnnotation: testMetricPropertyNamespaceName,
				metricPropertyMetricNameAnnotation:    testMetricPropertyMetricName,
			},
		},
	}
}

func metricPropertySummary(
	metricName string,
	dimensions map[string]ocicontrolcentersdk.DimensionValue,
) ocicontrolcentersdk.MetricPropertySummary {
	return ocicontrolcentersdk.MetricPropertySummary{
		MetricName: common.String(metricName),
		Dimensions: dimensions,
	}
}

func requireStringPtr(t *testing.T, label string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", label, *got, want)
	}
}

func requireCondition(
	t *testing.T,
	status shared.OSOKStatus,
	conditionType shared.OSOKConditionType,
	conditionStatus corev1.ConditionStatus,
) {
	t.Helper()
	for _, condition := range status.Conditions {
		if condition.Type == conditionType && condition.Status == conditionStatus {
			return
		}
	}
	t.Fatalf("condition %s/%s not found in %#v", conditionType, conditionStatus, status.Conditions)
}
