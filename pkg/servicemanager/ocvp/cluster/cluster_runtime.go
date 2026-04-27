/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cluster

import (
	"context"
	"fmt"
	"strings"

	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func init() {
	registerClusterRuntimeHooksMutator(func(_ *ClusterServiceManager, hooks *ClusterRuntimeHooks) {
		applyClusterRuntimeHooks(hooks)
	})
}

func applyClusterRuntimeHooks(hooks *ClusterRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = reviewedClusterRuntimeSemantics()
	hooks.Identity.GuardExistingBeforeCreate = guardClusterExistingBeforeCreate
}

func reviewedClusterRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newClusterRuntimeSemantics()
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable: []string{
			"clusterByolAllocationDetails",
			"definedTags",
			"displayName",
			"esxiSoftwareVersion",
			"freeformTags",
			"networkConfiguration",
			"vmwareSoftwareVersion",
		},
		ForceNew: []string{
			"capacityReservationId",
			"computeAvailabilityDomain",
			"datastoreClusterIds",
			"datastores",
			"esxiHostsCount",
			"initialCommitment",
			"initialHostOcpuCount",
			"initialHostShapeName",
			"initialVcfByolAllocationId",
			"instanceDisplayNamePrefix",
			"isShieldedInstanceEnabled",
			"sddcId",
			"workloadNetworkCidr",
		},
		ConflictsWith: map[string][]string{},
	}
	return semantics
}

func guardClusterExistingBeforeCreate(
	_ context.Context,
	resource *ocvpv1beta1.Cluster,
) (generatedruntime.ExistingBeforeCreateDecision, error) {
	if clusterIdentityResolutionRequiresDisplayName(resource) {
		return generatedruntime.ExistingBeforeCreateDecisionFail, fmt.Errorf("Cluster spec.displayName is required when no OCI identifier is recorded")
	}
	return generatedruntime.ExistingBeforeCreateDecisionAllow, nil
}

func clusterIdentityResolutionRequiresDisplayName(resource *ocvpv1beta1.Cluster) bool {
	if resource == nil {
		return false
	}
	if strings.TrimSpace(resource.Spec.DisplayName) != "" {
		return false
	}
	return currentClusterID(resource) == ""
}

func currentClusterID(resource *ocvpv1beta1.Cluster) string {
	if resource == nil {
		return ""
	}
	if ocid := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid)); ocid != "" {
		return ocid
	}
	return strings.TrimSpace(resource.Status.Id)
}
