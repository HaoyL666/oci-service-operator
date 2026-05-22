/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloud

import (
	"strings"

	"github.com/oracle/oci-service-operator/pkg/core"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	externalLocationSummariesMetadataSubscriptionServiceNameAnnotation = "multicloud.oracle.com/subscription-service-name"
	externalLocationSummariesMetadataCompartmentIDAnnotation           = "multicloud.oracle.com/compartment-id"
	externalLocationSummariesMetadataSubscriptionIDAnnotation          = "multicloud.oracle.com/subscription-id"
	externalLocationSummariesMetadataEntityTypeAnnotation              = "multicloud.oracle.com/entity-type"
	externalLocationSummariesMetadataLimitAnnotation                   = "multicloud.oracle.com/limit"
	externalLocationSummariesMetadataSortOrderAnnotation               = "multicloud.oracle.com/sort-order"
	externalLocationSummariesMetadataSortByAnnotation                  = "multicloud.oracle.com/sort-by"
)

var externalLocationSummariesMetadataFilterAnnotationKeys = []string{
	externalLocationSummariesMetadataSubscriptionServiceNameAnnotation,
	externalLocationSummariesMetadataCompartmentIDAnnotation,
	externalLocationSummariesMetadataSubscriptionIDAnnotation,
	externalLocationSummariesMetadataEntityTypeAnnotation,
	externalLocationSummariesMetadataLimitAnnotation,
	externalLocationSummariesMetadataSortOrderAnnotation,
	externalLocationSummariesMetadataSortByAnnotation,
}

func externalLocationSummariesMetadataReconcilePredicate() predicate.Predicate {
	return predicate.Or(
		core.ReconcilePredicate(),
		predicate.Funcs{
			UpdateFunc: externalLocationSummariesMetadataFilterAnnotationsChanged,
		},
	)
}

func externalLocationSummariesMetadataFilterAnnotationsChanged(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldAnnotations := e.ObjectOld.GetAnnotations()
	newAnnotations := e.ObjectNew.GetAnnotations()
	for _, key := range externalLocationSummariesMetadataFilterAnnotationKeys {
		if strings.TrimSpace(oldAnnotations[key]) != strings.TrimSpace(newAnnotations[key]) {
			return true
		}
	}
	return false
}
