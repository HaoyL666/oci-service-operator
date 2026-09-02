/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package mappedsecret

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticMappedSecretCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "MappedSecret", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, ocireplay.ModeReplay, filepath.Join("testdata", "recordings", "mappedsecret_synthetic_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.MappedSecret{Spec: networkfirewallv1beta1.MappedSecretSpec{NetworkFirewallPolicyId: "ocid1.networkfirewallpolicy.oc1..replay", Name: "osok_replay_mapped_secret", Type: "SSL_FORWARD_PROXY", Source: "OCI_VAULT", VaultSecretId: "<redacted>", VersionNumber: 1}}
	manager := &MappedSecretServiceManager{}
	hooks := newMappedSecretRuntimeHooks(manager, sdkClient)
	client := wrapMappedSecretGeneratedClient(hooks, defaultMappedSecretServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.MappedSecret](buildMappedSecretGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.MappedSecret]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: closeSession, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.MappedSecret) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.MappedSecret) error {
			if current.Status.Name != resource.Spec.Name || current.Status.VersionNumber != 1 {
				return fmt.Errorf("created MappedSecret status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.MappedSecret) { current.Spec.VersionNumber = 2 },
		ValidateUpdated: func(current *networkfirewallv1beta1.MappedSecret) error {
			if current.Status.VersionNumber != 2 {
				return fmt.Errorf("updated MappedSecret status = %+v", current.Status)
			}
			return nil
		},
	})
}
