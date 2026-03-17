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

// NatGatewayReconciler reconciles a NatGateway object.
type NatGatewayReconciler struct {
	DisplayNameOld string
	Reconciler     *osokcore.BaseReconciler
}

// +kubebuilder:rbac:groups=core.oracle.com,resources=natgateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.oracle.com,resources=natgateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.oracle.com,resources=natgateways/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NatGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	natGateway := &corev1beta1.NatGateway{}
	return r.Reconciler.Reconcile(ctx, req, natGateway)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NatGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.NatGateway{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
