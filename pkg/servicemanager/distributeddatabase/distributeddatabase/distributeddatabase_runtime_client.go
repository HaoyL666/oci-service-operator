/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package distributeddatabase

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	distributeddatabasesdk "github.com/oracle/oci-go-sdk/v65/distributeddatabase"
	distributeddatabasev1beta1 "github.com/oracle/oci-service-operator/api/distributeddatabase/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
)

func init() {
	registerDistributedDatabaseRuntimeHooksMutator(func(manager *DistributedDatabaseServiceManager, hooks *DistributedDatabaseRuntimeHooks) {
		applyDistributedDatabaseRuntimeHooks(manager, hooks)
	})
}

func applyDistributedDatabaseRuntimeHooks(
	manager *DistributedDatabaseServiceManager,
	hooks *DistributedDatabaseRuntimeHooks,
) {
	if hooks == nil {
		return
	}

	var credentialClient credhelper.CredentialClient
	if manager != nil {
		credentialClient = manager.CredentialClient
	}

	hooks.Semantics = reviewedDistributedDatabaseRuntimeSemantics()
	hooks.Get.Fields = reviewedDistributedDatabaseGetFields()
	hooks.List.Fields = reviewedDistributedDatabaseListFields()
	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *distributeddatabasev1beta1.DistributedDatabase,
		namespace string,
	) (any, error) {
		return buildDistributedDatabaseCreateBody(ctx, credentialClient, resource, namespace)
	}
	hooks.BuildUpdateBody = func(
		_ context.Context,
		resource *distributeddatabasev1beta1.DistributedDatabase,
		_ string,
		currentResponse any,
	) (any, bool, error) {
		return buildDistributedDatabaseUpdateBody(resource, currentResponse)
	}
	hooks.ParityHooks.UnsupportedDriftEquivalent = distributedDatabaseUnsupportedDriftEquivalent
	hooks.TrackedRecreate.ClearTrackedIdentity = clearTrackedDistributedDatabaseIdentity
}

func distributedDatabaseUnsupportedDriftEquivalent(path string, desired, observed any) (bool, bool) {
	switch path {
	case "catalogDetails":
		return true, distributedDatabaseDesiredSubsetMatches(path, desired, observed)
	default:
		return false, false
	}
}

func distributedDatabaseDesiredSubsetMatches(rootPath string, desired, observed any) bool {
	switch desiredValue := desired.(type) {
	case map[string]any:
		observedValue, ok := observed.(map[string]any)
		if !ok {
			return false
		}
		for key, desiredChild := range desiredValue {
			if distributedDatabaseUnobservableCreateField(rootPath, key) {
				continue
			}
			observedChild, found := distributedDatabaseMapValue(observedValue, key)
			if !found || !distributedDatabaseDesiredSubsetMatches(rootPath, desiredChild, observedChild) {
				return false
			}
		}
		return true
	case []any:
		observedValue, ok := observed.([]any)
		if !ok || len(desiredValue) != len(observedValue) {
			return false
		}
		for index := range desiredValue {
			if !distributedDatabaseDesiredSubsetMatches(rootPath, desiredValue[index], observedValue[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(desired, observed)
	}
}

func distributedDatabaseUnobservableCreateField(_ string, field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "adminpassword", "tdewalletpassword", "dbstoragevaultdetails", "vmclusterdetails":
		return true
	default:
		return false
	}
}

func distributedDatabaseMapValue(values map[string]any, key string) (any, bool) {
	if value, ok := values[key]; ok {
		return value, true
	}
	normalized := strings.ToLower(strings.TrimSpace(key))
	for candidate, value := range values {
		if strings.ToLower(strings.TrimSpace(candidate)) == normalized {
			return value, true
		}
	}
	return nil, false
}

func reviewedDistributedDatabaseRuntimeSemantics() *generatedruntime.Semantics {
	semantics := newDistributedDatabaseRuntimeSemantics()
	semantics.Lifecycle = generatedruntime.LifecycleSemantics{
		ProvisioningStates: []string{"CREATING"},
		UpdatingStates:     []string{"UPDATING"},
		ActiveStates:       []string{"ACTIVE", "INACTIVE"},
	}
	semantics.Delete = generatedruntime.DeleteSemantics{
		Policy:         "required",
		PendingStates:  []string{"DELETING"},
		TerminalStates: []string{"DELETED", "NOT_FOUND"},
	}
	semantics.List = &generatedruntime.ListSemantics{
		ResponseItemsField: "Items",
		MatchFields:        []string{"compartmentId", "displayName", "prefix", "dbDeploymentType", "lifecycleState"},
	}
	semantics.Mutation = generatedruntime.MutationSemantics{
		Mutable:       []string{"definedTags", "displayName", "freeformTags"},
		ForceNew:      []string{"compartmentId"},
		ConflictsWith: map[string][]string{},
	}
	semantics.AuxiliaryOperations = nil
	return semantics
}

func reviewedDistributedDatabaseGetFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "DistributedDatabaseId", RequestName: "distributedDatabaseId", Contribution: "path", PreferResourceID: true},
	}
}

func reviewedDistributedDatabaseListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "CompartmentId", RequestName: "compartmentId", Contribution: "query"},
		{FieldName: "LifecycleState", RequestName: "lifecycleState", Contribution: "query"},
		{FieldName: "Limit", RequestName: "limit", Contribution: "query"},
		{FieldName: "Page", RequestName: "page", Contribution: "query"},
		{FieldName: "SortOrder", RequestName: "sortOrder", Contribution: "query"},
		{FieldName: "SortBy", RequestName: "sortBy", Contribution: "query"},
		{FieldName: "DisplayName", RequestName: "displayName", Contribution: "query"},
		{FieldName: "DbDeploymentType", RequestName: "dbDeploymentType", Contribution: "query"},
	}
}

func buildDistributedDatabaseCreateBody(
	ctx context.Context,
	credentialClient credhelper.CredentialClient,
	resource *distributeddatabasev1beta1.DistributedDatabase,
	namespace string,
) (distributeddatabasesdk.CreateDistributedDatabaseDetails, error) {
	if resource == nil {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, fmt.Errorf("DistributedDatabase resource is nil")
	}

	resolvedSpec, err := generatedruntime.ResolveSpecValueWithBoolFields(resource, ctx, credentialClient, namespace)
	if err != nil {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, err
	}

	createMap, ok := resolvedSpec.(map[string]any)
	if !ok {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, fmt.Errorf("resolved DistributedDatabase spec is %T, want map[string]any", resolvedSpec)
	}
	if err := normalizeDistributedDatabaseCreateBody(createMap); err != nil {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, err
	}

	payload, err := json.Marshal(createMap)
	if err != nil {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, fmt.Errorf("marshal resolved DistributedDatabase create body: %w", err)
	}

	var details distributeddatabasesdk.CreateDistributedDatabaseDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return distributeddatabasesdk.CreateDistributedDatabaseDetails{}, fmt.Errorf("decode DistributedDatabase create request body: %w", err)
	}
	normalizeDistributedDatabaseCreateDetails(resource.Spec, &details)

	return details, nil
}

func normalizeDistributedDatabaseCreateBody(body map[string]any) error {
	if err := normalizeDistributedDatabaseMapSlice(body, "catalogDetails", "catalogDetails", func(catalog map[string]any, path string) error {
		return normalizeDistributedDatabaseReplicaSet(catalog, path, "database")
	}); err != nil {
		return err
	}
	if err := normalizeDistributedDatabaseMapSlice(body, "compositeRaftShardSpaces", "compositeRaftShardSpaces", func(shardSpace map[string]any, path string) error {
		return normalizeDistributedDatabaseMapSlice(shardSpace, "raftClusters", path+".raftClusters", func(cluster map[string]any, clusterPath string) error {
			return normalizeDistributedDatabaseDatabaseSlice(cluster, "databases", clusterPath+".databases")
		})
	}); err != nil {
		return err
	}
	if err := normalizeDistributedDatabaseMapSlice(body, "compositeDataGuardShardSpaces", "compositeDataGuardShardSpaces", func(shardSpace map[string]any, path string) error {
		return normalizeDistributedDatabaseReplicaSet(shardSpace, path, "databases")
	}); err != nil {
		return err
	}
	if err := normalizeDistributedDatabaseMapSlice(body, "systemRaftClusters", "systemRaftClusters", func(cluster map[string]any, path string) error {
		return normalizeDistributedDatabaseDatabaseSlice(cluster, "databases", path+".databases")
	}); err != nil {
		return err
	}
	if raw, ok := body["systemDataGuardDatabases"]; ok && raw != nil {
		systemDatabases, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("resolved DistributedDatabase systemDataGuardDatabases is %T, want map[string]any", raw)
		}
		if err := normalizeDistributedDatabaseReplicaSet(systemDatabases, "systemDataGuardDatabases", "databases"); err != nil {
			return err
		}
	}
	return normalizeDistributedDatabaseMapSlice(body, "userShardSpaces", "userShardSpaces", func(shardSpace map[string]any, path string) error {
		return normalizeDistributedDatabaseReplicaSet(shardSpace, path, "database")
	})
}

func normalizeDistributedDatabaseMapSlice(parent map[string]any, field, path string, normalize func(map[string]any, string) error) error {
	rawItems, ok := parent[field]
	if !ok || rawItems == nil {
		return nil
	}
	items, ok := rawItems.([]any)
	if !ok {
		return fmt.Errorf("resolved DistributedDatabase %s is %T, want []any", path, rawItems)
	}
	for index, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return fmt.Errorf("resolved DistributedDatabase %s[%d] is %T, want map[string]any", path, index, rawItem)
		}
		if err := normalize(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	return nil
}

func normalizeDistributedDatabaseReplicaSet(parent map[string]any, path, databaseField string) error {
	if err := normalizeDistributedDatabaseReplica(parent, "originalReplica", path+".originalReplica", databaseField); err != nil {
		return err
	}
	return normalizeDistributedDatabaseMapSlice(parent, "dataGuardReplicas", path+".dataGuardReplicas", func(replica map[string]any, replicaPath string) error {
		return normalizeDistributedDatabaseReplicaDatabases(replica, replicaPath, databaseField)
	})
}

func normalizeDistributedDatabaseReplica(parent map[string]any, field, path, databaseField string) error {
	rawReplica, ok := parent[field]
	if !ok || rawReplica == nil {
		return fmt.Errorf("resolved DistributedDatabase %s is required", path)
	}
	replica, ok := rawReplica.(map[string]any)
	if !ok {
		return fmt.Errorf("resolved DistributedDatabase %s is %T, want map[string]any", path, rawReplica)
	}
	return normalizeDistributedDatabaseReplicaDatabases(replica, path, databaseField)
}

func normalizeDistributedDatabaseReplicaDatabases(replica map[string]any, path, databaseField string) error {
	if databaseField == "database" {
		rawDatabase, ok := replica[databaseField]
		if !ok || rawDatabase == nil {
			return fmt.Errorf("resolved DistributedDatabase %s.database is required", path)
		}
		database, ok := rawDatabase.(map[string]any)
		if !ok {
			return fmt.Errorf("resolved DistributedDatabase %s.database is %T, want map[string]any", path, rawDatabase)
		}
		return normalizeDistributedDatabaseDatabase(database, path+".database")
	}
	return normalizeDistributedDatabaseDatabaseSlice(replica, databaseField, path+"."+databaseField)
}

func normalizeDistributedDatabaseDatabaseSlice(parent map[string]any, field, path string) error {
	return normalizeDistributedDatabaseMapSlice(parent, field, path, func(database map[string]any, databasePath string) error {
		return normalizeDistributedDatabaseDatabase(database, databasePath)
	})
}

func normalizeDistributedDatabaseDatabase(database map[string]any, path string) error {
	if rawJSONData, ok := database["jsonData"]; ok {
		delete(database, "jsonData")
		if rawJSONData != nil {
			jsonData, ok := rawJSONData.(string)
			if !ok {
				return fmt.Errorf("resolved DistributedDatabase %s.jsonData is %T, want string", path, rawJSONData)
			}
			if strings.TrimSpace(jsonData) != "" {
				var payload map[string]any
				if err := json.Unmarshal([]byte(jsonData), &payload); err != nil || payload == nil {
					return fmt.Errorf("resolved DistributedDatabase %s.jsonData must be a JSON object", path)
				}
				for key, value := range payload {
					if existing, exists := database[key]; exists && !reflect.DeepEqual(existing, value) {
						return fmt.Errorf("resolved DistributedDatabase %s.jsonData conflicts with explicit %s", path, key)
					}
					database[key] = value
				}
			}
		}
	}

	source, err := normalizedDistributedDatabaseSource(database["source"])
	if err != nil {
		return fmt.Errorf("resolved DistributedDatabase %s: %w", path, err)
	}
	database["source"] = source
	return nil
}

func normalizedDistributedDatabaseSource(raw any) (string, error) {
	source, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("source is %T, want string", raw)
	}

	switch normalized := strings.ToUpper(strings.TrimSpace(source)); normalized {
	case "XS_NEW_VAULT_AND_CLUSTER", "XS_NEW_CLUSTER", "XS_EXISTING_CLUSTER", "XD_EXISTING_CLUSTER", "EXISTING_DB_HOME":
		return normalized, nil
	case "":
		return "", fmt.Errorf("source is required")
	default:
		return "", fmt.Errorf("unsupported source")
	}
}

func normalizeDistributedDatabaseCreateDetails(
	spec distributeddatabasev1beta1.DistributedDatabaseSpec,
	details *distributeddatabasesdk.CreateDistributedDatabaseDetails,
) {
	if details == nil {
		return
	}

	if reflect.ValueOf(spec.DbBackupConfig).IsZero() {
		details.DbBackupConfig = nil
	} else {
		applyDistributedDatabaseBackupConfigBoolPointers(spec.DbBackupConfig, details.DbBackupConfig)
	}

	for index := range spec.CatalogDetails {
		if index >= len(details.CatalogDetails) {
			break
		}
		catalog := &details.CatalogDetails[index]
		if catalog.OriginalReplica != nil {
			catalog.OriginalReplica.Database = applyDistributedDatabaseCatalogVmClusterBoolPointers(
				spec.CatalogDetails[index].OriginalReplica.Database.VmClusterDetails.IsDiagnosticsEventsEnabled,
				spec.CatalogDetails[index].OriginalReplica.Database.VmClusterDetails.IsHealthMonitoringEnabled,
				spec.CatalogDetails[index].OriginalReplica.Database.VmClusterDetails.IsIncidentLogsEnabled,
				catalog.OriginalReplica.Database,
			)
		}
		for replicaIndex := range spec.CatalogDetails[index].DataGuardReplicas {
			if replicaIndex >= len(catalog.DataGuardReplicas) {
				break
			}
			catalog.DataGuardReplicas[replicaIndex].Database = applyDistributedDatabaseCatalogVmClusterBoolPointers(
				spec.CatalogDetails[index].DataGuardReplicas[replicaIndex].Database.VmClusterDetails.IsDiagnosticsEventsEnabled,
				spec.CatalogDetails[index].DataGuardReplicas[replicaIndex].Database.VmClusterDetails.IsHealthMonitoringEnabled,
				spec.CatalogDetails[index].DataGuardReplicas[replicaIndex].Database.VmClusterDetails.IsIncidentLogsEnabled,
				catalog.DataGuardReplicas[replicaIndex].Database,
			)
		}
	}
}

func applyDistributedDatabaseCatalogVmClusterBoolPointers(
	isDiagnosticsEventsEnabled bool,
	isHealthMonitoringEnabled bool,
	isIncidentLogsEnabled bool,
	database distributeddatabasesdk.CreateDistributedDatabaseCatalogDatabaseDetails,
) distributeddatabasesdk.CreateDistributedDatabaseCatalogDatabaseDetails {
	switch current := database.(type) {
	case distributeddatabasesdk.CreateDistributedDatabaseCatalogWithExadbXsNewVaultAndClusterDetails:
		applyDistributedDatabaseVmClusterBoolPointers(isDiagnosticsEventsEnabled, isHealthMonitoringEnabled, isIncidentLogsEnabled, current.VmClusterDetails)
		return current
	case distributeddatabasesdk.CreateDistributedDatabaseCatalogWithExadbXsNewClusterDetails:
		applyDistributedDatabaseVmClusterBoolPointers(isDiagnosticsEventsEnabled, isHealthMonitoringEnabled, isIncidentLogsEnabled, current.VmClusterDetails)
		return current
	default:
		return database
	}
}

func applyDistributedDatabaseVmClusterBoolPointers(
	isDiagnosticsEventsEnabled bool,
	isHealthMonitoringEnabled bool,
	isIncidentLogsEnabled bool,
	details *distributeddatabasesdk.VmClusterDetails,
) {
	if details == nil {
		return
	}
	details.IsDiagnosticsEventsEnabled = common.Bool(isDiagnosticsEventsEnabled)
	details.IsHealthMonitoringEnabled = common.Bool(isHealthMonitoringEnabled)
	details.IsIncidentLogsEnabled = common.Bool(isIncidentLogsEnabled)
}
func applyDistributedDatabaseBackupConfigBoolPointers(
	spec distributeddatabasev1beta1.DistributedDatabaseDbBackupConfig,
	details *distributeddatabasesdk.DistributedDbBackupConfig,
) {
	if details == nil {
		return
	}
	details.IsAutoBackupEnabled = common.Bool(spec.IsAutoBackupEnabled)
	details.CanRunImmediateFullBackup = common.Bool(spec.CanRunImmediateFullBackup)
	details.IsRemoteBackupEnabled = common.Bool(spec.IsRemoteBackupEnabled)

	for index := range spec.BackupDestinationDetails {
		if index >= len(details.BackupDestinationDetails) {
			break
		}
		details.BackupDestinationDetails[index].IsRemote = common.Bool(spec.BackupDestinationDetails[index].IsRemote)
	}
}

func buildDistributedDatabaseUpdateBody(
	resource *distributeddatabasev1beta1.DistributedDatabase,
	currentResponse any,
) (distributeddatabasesdk.UpdateDistributedDatabaseDetails, bool, error) {
	if resource == nil {
		return distributeddatabasesdk.UpdateDistributedDatabaseDetails{}, false, fmt.Errorf("DistributedDatabase resource is nil")
	}

	current, ok := distributedDatabaseFromResponse(currentResponse)
	if !ok {
		return distributeddatabasesdk.UpdateDistributedDatabaseDetails{}, false, fmt.Errorf("current DistributedDatabase response does not expose a DistributedDatabase body")
	}

	updateDetails := distributeddatabasesdk.UpdateDistributedDatabaseDetails{}
	updateNeeded := false

	if desired, ok := desiredDistributedDatabaseStringUpdate(resource.Spec.DisplayName, current.DisplayName); ok {
		updateDetails.DisplayName = desired
		updateNeeded = true
	}

	desiredFreeformTags := desiredDistributedDatabaseFreeformTagsForUpdate(resource.Spec.FreeformTags, current.FreeformTags)
	if !reflect.DeepEqual(current.FreeformTags, desiredFreeformTags) {
		updateDetails.FreeformTags = desiredFreeformTags
		updateNeeded = true
	}

	desiredDefinedTags := desiredDistributedDatabaseDefinedTagsForUpdate(resource.Spec.DefinedTags, current.DefinedTags)
	if !reflect.DeepEqual(current.DefinedTags, desiredDefinedTags) {
		updateDetails.DefinedTags = desiredDefinedTags
		updateNeeded = true
	}

	if !updateNeeded {
		return distributeddatabasesdk.UpdateDistributedDatabaseDetails{}, false, nil
	}
	return updateDetails, true, nil
}

func distributedDatabaseFromResponse(response any) (distributeddatabasesdk.DistributedDatabase, bool) {
	switch current := response.(type) {
	case distributeddatabasesdk.CreateDistributedDatabaseResponse:
		return current.DistributedDatabase, true
	case *distributeddatabasesdk.CreateDistributedDatabaseResponse:
		if current == nil {
			return distributeddatabasesdk.DistributedDatabase{}, false
		}
		return current.DistributedDatabase, true
	case distributeddatabasesdk.GetDistributedDatabaseResponse:
		return current.DistributedDatabase, true
	case *distributeddatabasesdk.GetDistributedDatabaseResponse:
		if current == nil {
			return distributeddatabasesdk.DistributedDatabase{}, false
		}
		return current.DistributedDatabase, true
	case distributeddatabasesdk.UpdateDistributedDatabaseResponse:
		return current.DistributedDatabase, true
	case *distributeddatabasesdk.UpdateDistributedDatabaseResponse:
		if current == nil {
			return distributeddatabasesdk.DistributedDatabase{}, false
		}
		return current.DistributedDatabase, true
	case distributeddatabasesdk.DistributedDatabase:
		return current, true
	case *distributeddatabasesdk.DistributedDatabase:
		if current == nil {
			return distributeddatabasesdk.DistributedDatabase{}, false
		}
		return *current, true
	case distributeddatabasesdk.DistributedDatabaseSummary:
		return distributedDatabaseFromSummary(current), true
	case *distributeddatabasesdk.DistributedDatabaseSummary:
		if current == nil {
			return distributeddatabasesdk.DistributedDatabase{}, false
		}
		return distributedDatabaseFromSummary(*current), true
	default:
		return distributeddatabasesdk.DistributedDatabase{}, false
	}
}

func distributedDatabaseFromSummary(
	summary distributeddatabasesdk.DistributedDatabaseSummary,
) distributeddatabasesdk.DistributedDatabase {
	return distributeddatabasesdk.DistributedDatabase{
		Id:                             summary.Id,
		CompartmentId:                  summary.CompartmentId,
		DisplayName:                    summary.DisplayName,
		TimeCreated:                    summary.TimeCreated,
		TimeUpdated:                    summary.TimeUpdated,
		DatabaseVersion:                summary.DatabaseVersion,
		LifecycleState:                 summary.LifecycleState,
		LifecycleDetails:               summary.LifecycleDetails,
		Prefix:                         summary.Prefix,
		PrivateEndpointIds:             append([]string(nil), summary.PrivateEndpointIds...),
		ShardingMethod:                 distributeddatabasesdk.DistributedDatabaseShardingMethodEnum(summary.ShardingMethod),
		CharacterSet:                   summary.CharacterSet,
		NcharacterSet:                  summary.NcharacterSet,
		ListenerPort:                   summary.ListenerPort,
		OnsPortLocal:                   summary.OnsPortLocal,
		OnsPortRemote:                  summary.OnsPortRemote,
		DbDeploymentType:               distributeddatabasesdk.DistributedDatabaseDbDeploymentTypeEnum(summary.DbDeploymentType),
		NotificationTopicIds:           append([]string(nil), summary.NotificationTopicIds...),
		ListenerPortTls:                summary.ListenerPortTls,
		SystemChunkCount:               summary.SystemChunkCount,
		AutoResourceManagementConfig:   summary.AutoResourceManagementConfig,
		SystemRaftReplicationUnitCount: summary.SystemRaftReplicationUnitCount,
		ReplicationMethod:              distributeddatabasesdk.DistributedDatabaseReplicationMethodEnum(summary.ReplicationMethod),
		Metadata:                       summary.Metadata,
		FreeformTags:                   cloneDistributedDatabaseStringMap(summary.FreeformTags),
		DefinedTags:                    cloneDistributedDatabaseDefinedTags(summary.DefinedTags),
		SystemTags:                     cloneDistributedDatabaseDefinedTags(summary.SystemTags),
	}
}

func clearTrackedDistributedDatabaseIdentity(resource *distributeddatabasev1beta1.DistributedDatabase) {
	if resource == nil {
		return
	}
	resource.Status.Id = ""
	resource.Status.OsokStatus = shared.OSOKStatus{}
}

func desiredDistributedDatabaseStringUpdate(spec string, current *string) (*string, bool) {
	currentValue := ""
	if current != nil {
		currentValue = *current
	}
	if spec == currentValue {
		return nil, false
	}
	if spec == "" && current == nil {
		return nil, false
	}
	return common.String(spec), true
}

func desiredDistributedDatabaseFreeformTagsForUpdate(spec map[string]string, current map[string]string) map[string]string {
	if spec != nil {
		return cloneDistributedDatabaseStringMap(spec)
	}
	if current != nil {
		return map[string]string{}
	}
	return nil
}

func desiredDistributedDatabaseDefinedTagsForUpdate(
	spec map[string]shared.MapValue,
	current map[string]map[string]interface{},
) map[string]map[string]interface{} {
	if spec != nil {
		return *util.ConvertToOciDefinedTags(&spec)
	}
	if current != nil {
		return map[string]map[string]interface{}{}
	}
	return nil
}

func cloneDistributedDatabaseStringMap(spec map[string]string) map[string]string {
	if spec == nil {
		return nil
	}
	out := make(map[string]string, len(spec))
	for key, value := range spec {
		out[key] = value
	}
	return out
}

func cloneDistributedDatabaseDefinedTags(spec map[string]map[string]interface{}) map[string]map[string]interface{} {
	if spec == nil {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(spec))
	for namespace, values := range spec {
		if values == nil {
			out[namespace] = nil
			continue
		}
		clonedValues := make(map[string]interface{}, len(values))
		for key, value := range values {
			clonedValues[key] = value
		}
		out[namespace] = clonedValues
	}
	return out
}
