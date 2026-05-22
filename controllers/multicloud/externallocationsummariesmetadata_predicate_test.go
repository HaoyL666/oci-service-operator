/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloud

import (
	"testing"

	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestExternalLocationSummariesMetadataPredicateAllowsFilterAnnotationChanges(t *testing.T) {
	t.Parallel()

	pred := externalLocationSummariesMetadataReconcilePredicate()
	for _, key := range externalLocationSummariesMetadataFilterAnnotationKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			oldObj := newExternalLocationSummariesMetadataPredicateObject()
			newObj := oldObj.DeepCopy()
			newObj.Annotations[key] = "updated"

			if !pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
				t.Fatalf("Update() should allow %q annotation changes", key)
			}
		})
	}
}

func TestExternalLocationSummariesMetadataPredicateAllowsAnnotationCorrection(t *testing.T) {
	t.Parallel()

	oldObj := newExternalLocationSummariesMetadataPredicateObject()
	oldObj.Annotations[externalLocationSummariesMetadataSubscriptionServiceNameAnnotation] = "not-a-service"
	newObj := oldObj.DeepCopy()
	newObj.Annotations[externalLocationSummariesMetadataSubscriptionServiceNameAnnotation] = "ORACLEDBATAZURE"

	if !externalLocationSummariesMetadataReconcilePredicate().Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
		t.Fatal("Update() should allow correcting an invalid filter annotation without a generation change")
	}
}

func TestExternalLocationSummariesMetadataPredicateRejectsUnrelatedAnnotationChange(t *testing.T) {
	t.Parallel()

	oldObj := newExternalLocationSummariesMetadataPredicateObject()
	newObj := oldObj.DeepCopy()
	newObj.Annotations["example.com/unrelated"] = "updated"

	if externalLocationSummariesMetadataReconcilePredicate().Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
		t.Fatal("Update() should reject unrelated annotation-only changes")
	}
}

func newExternalLocationSummariesMetadataPredicateObject() *multicloudv1beta1.ExternalLocationSummariesMetadata {
	return &multicloudv1beta1.ExternalLocationSummariesMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "external-location-summaries-metadata",
			Namespace:  "default",
			Generation: 1,
			Annotations: map[string]string{
				externalLocationSummariesMetadataSubscriptionServiceNameAnnotation: "ORACLEDBATAWS",
				externalLocationSummariesMetadataCompartmentIDAnnotation:           "ocid1.compartment.oc1..multicloud",
				externalLocationSummariesMetadataSubscriptionIDAnnotation:          "ocid1.multicloudsubscription.oc1..subscription",
				externalLocationSummariesMetadataEntityTypeAnnotation:              "dbsystem",
				externalLocationSummariesMetadataLimitAnnotation:                   "25",
				externalLocationSummariesMetadataSortOrderAnnotation:               "ASC",
				externalLocationSummariesMetadataSortByAnnotation:                  "displayName",
			},
		},
	}
}
