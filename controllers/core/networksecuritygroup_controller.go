/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package core

import (
	"context"

	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	osokcore "github.com/oracle/oci-service-operator/pkg/core"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NetworkSecurityGroupReconciler reconciles a NetworkSecurityGroup object.
type NetworkSecurityGroupReconciler struct {
	DisplayNameOld string
	Reconciler     *osokcore.BaseReconciler
}

// +kubebuilder:rbac:groups=core.oracle.com,resources=networksecuritygroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.oracle.com,resources=networksecuritygroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.oracle.com,resources=networksecuritygroups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NetworkSecurityGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	networkSecurityGroup := &corev1beta1.NetworkSecurityGroup{}
	return r.Reconciler.Reconcile(ctx, req, networkSecurityGroup)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkSecurityGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.NetworkSecurityGroup{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
