package alarmsuppression

import (
	"context"
	"reflect"
	"testing"

	monitoringsdk "github.com/oracle/oci-go-sdk/v65/monitoring"
	monitoringv1beta1 "github.com/oracle/oci-service-operator/api/monitoring/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyAlarmSuppressionRuntimeHooksBuildsCreateBodyWithUpgradeFields(t *testing.T) {
	t.Parallel()

	hooks := newAlarmSuppressionDefaultRuntimeHooks(monitoringsdk.MonitoringClient{})
	applyAlarmSuppressionRuntimeHooks(&AlarmSuppressionServiceManager{}, &hooks)

	if hooks.BuildCreateBody == nil {
		t.Fatal("hooks.BuildCreateBody = nil, want custom create builder")
	}

	body, err := hooks.BuildCreateBody(context.Background(), newAlarmSuppressionDimensionTestResource(), "default")
	if err != nil {
		t.Fatalf("hooks.BuildCreateBody() error = %v", err)
	}

	details, ok := body.(monitoringsdk.CreateAlarmSuppressionDetails)
	if !ok {
		t.Fatalf("hooks.BuildCreateBody() body type = %T, want monitoring.CreateAlarmSuppressionDetails", body)
	}
	if details.Level != monitoringsdk.AlarmSuppressionLevelDimension {
		t.Fatalf("Level = %q, want DIMENSION", details.Level)
	}
	if !reflect.DeepEqual(details.Dimensions, map[string]string{"resourceId": "ocid1.instance.oc1..example"}) {
		t.Fatalf("Dimensions = %#v, want preserved dimension filter", details.Dimensions)
	}
	if len(details.SuppressionConditions) != 1 {
		t.Fatalf("len(SuppressionConditions) = %d, want 1", len(details.SuppressionConditions))
	}
	recurrence, ok := details.SuppressionConditions[0].(monitoringsdk.Recurrence)
	if !ok {
		t.Fatalf("SuppressionConditions[0] type = %T, want monitoring.Recurrence", details.SuppressionConditions[0])
	}
	if recurrence.SuppressionRecurrence == nil || *recurrence.SuppressionRecurrence != "FREQ=DAILY;BYHOUR=10" {
		t.Fatalf("SuppressionRecurrence = %#v, want FREQ=DAILY;BYHOUR=10", recurrence.SuppressionRecurrence)
	}
	if recurrence.SuppressionDuration == nil || *recurrence.SuppressionDuration != "PT1H" {
		t.Fatalf("SuppressionDuration = %#v, want PT1H", recurrence.SuppressionDuration)
	}
}

func TestBuildAlarmSuppressionCreateDetailsAllowsAlarmLevelWithoutDimensions(t *testing.T) {
	t.Parallel()

	details, err := buildAlarmSuppressionCreateDetails(context.Background(), nil, newAlarmSuppressionAlarmLevelTestResource(), "default")
	if err != nil {
		t.Fatalf("buildAlarmSuppressionCreateDetails() error = %v", err)
	}
	if details.Level != monitoringsdk.AlarmSuppressionLevelAlarm {
		t.Fatalf("Level = %q, want ALARM", details.Level)
	}
	if details.Dimensions != nil {
		t.Fatalf("Dimensions = %#v, want nil when alarm-level suppression does not scope by dimension", details.Dimensions)
	}
}

func newAlarmSuppressionDimensionTestResource() *monitoringv1beta1.AlarmSuppression {
	return &monitoringv1beta1.AlarmSuppression{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alarm-suppression-sample",
			Namespace: "default",
		},
		Spec: monitoringv1beta1.AlarmSuppressionSpec{
			AlarmSuppressionTarget: monitoringv1beta1.AlarmSuppressionTarget{
				TargetType: "ALARM",
				AlarmId:    "ocid1.alarm.oc1..example",
			},
			DisplayName:       "alarm-suppression-sample",
			TimeSuppressFrom:  "2026-04-27T10:00:00Z",
			TimeSuppressUntil: "2026-04-27T11:00:00Z",
			Level:             "DIMENSION",
			Dimensions: map[string]string{
				"resourceId": "ocid1.instance.oc1..example",
			},
			SuppressionConditions: []monitoringv1beta1.AlarmSuppressionSuppressionCondition{
				{
					ConditionType:         "RECURRENCE",
					SuppressionRecurrence: "FREQ=DAILY;BYHOUR=10",
					SuppressionDuration:   "PT1H",
				},
			},
		},
	}
}

func newAlarmSuppressionAlarmLevelTestResource() *monitoringv1beta1.AlarmSuppression {
	resource := newAlarmSuppressionDimensionTestResource()
	resource.Spec.Level = "ALARM"
	resource.Spec.Dimensions = nil
	resource.Spec.SuppressionConditions = nil
	return resource
}
