package connectionassignment

import (
	"context"
	"testing"

	goldengatev1beta1 "github.com/oracle/oci-service-operator/api/goldengate/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

type connectionAssignmentLifecycleClient struct {
	createOrUpdateCalls int
	deleteCalls         int
}

func (f *connectionAssignmentLifecycleClient) CreateOrUpdate(_ context.Context, resource *goldengatev1beta1.ConnectionAssignment, _ ctrl.Request) (servicemanager.OSOKResponse, error) {
	f.createOrUpdateCalls++
	resource.Status.Id = "ocid1.connectionassignment.oc1..example"
	resource.Status.OsokStatus.Ocid = shared.OCID(resource.Status.Id)
	resource.Status.ConnectionId = resource.Spec.ConnectionId
	resource.Status.ConnectionType = "ORACLE"
	resource.Status.DeploymentId = resource.Spec.DeploymentId
	resource.Status.CompartmentId = "ocid1.compartment.oc1..example"
	resource.Status.LifecycleState = "ACTIVE"
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (f *connectionAssignmentLifecycleClient) Delete(_ context.Context, resource *goldengatev1beta1.ConnectionAssignment) (bool, error) {
	f.deleteCalls++
	if f.deleteCalls == 1 {
		resource.Status.LifecycleState = "DELETING"
		return false, nil
	}
	resource.Status.LifecycleState = "DELETED"
	return true, nil
}

func TestConnectionAssignmentLifecycleDelegatesStatusAndConfirmedDelete(t *testing.T) {
	t.Parallel()

	client := &connectionAssignmentLifecycleClient{}
	manager := (&ConnectionAssignmentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")}}).WithClient(client)
	resource := &goldengatev1beta1.ConnectionAssignment{Spec: goldengatev1beta1.ConnectionAssignmentSpec{
		ConnectionId: "ocid1.connection.oc1..example",
		DeploymentId: "ocid1.goldengatedeployment.oc1..example",
	}}

	response, err := manager.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil || !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, error = %v", response, err)
	}
	if resource.Status.ConnectionType != "ORACLE" || resource.Status.LifecycleState != "ACTIVE" || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("created ConnectionAssignment status = %#v", resource.Status)
	}
	deleted, err := manager.Delete(context.Background(), resource)
	if err != nil || deleted || resource.Status.LifecycleState != "DELETING" {
		t.Fatalf("first Delete() deleted = %t, error = %v, status = %#v; want pending confirmed delete", deleted, err, resource.Status)
	}
	deleted, err = manager.Delete(context.Background(), resource)
	if err != nil || !deleted || resource.Status.LifecycleState != "DELETED" {
		t.Fatalf("second Delete() deleted = %t, error = %v, status = %#v; want confirmed delete", deleted, err, resource.Status)
	}
	if client.createOrUpdateCalls != 1 || client.deleteCalls != 2 {
		t.Fatalf("lifecycle calls createOrUpdate=%d delete=%d, want 1 and 2", client.createOrUpdateCalls, client.deleteCalls)
	}
}
