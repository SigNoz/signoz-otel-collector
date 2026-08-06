package schemamigrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEnvironmentDependencyGraphMigration(t *testing.T) {
	var migration *SchemaMigrationRecord
	for idx := range TracesMigrations {
		if TracesMigrations[idx].MigrationID == 1015 {
			migration = &TracesMigrations[idx]
			break
		}
	}
	require.NotNil(t, migration)
	require.Len(t, migration.UpItems, 3)
	require.Len(t, migration.DownItems, 3)

	wantViews := map[string]bool{
		"dependency_graph_minutes_db_calls_mv_v2":        false,
		"dependency_graph_minutes_messaging_calls_mv_v2": false,
		"dependency_graph_minutes_service_calls_mv_v2":   false,
	}
	current := "['deployment.environment.name']"
	old := "['deployment.environment']"
	for _, item := range migration.UpItems {
		operation, ok := item.(ModifyQueryMaterializedViewOperation)
		require.True(t, ok)
		_, expected := wantViews[operation.ViewName]
		assert.True(t, expected, operation.ViewName)
		wantViews[operation.ViewName] = true

		currentIndex := strings.Index(operation.Query, current)
		oldIndex := strings.Index(operation.Query, old)
		require.NotEqual(t, -1, currentIndex)
		require.NotEqual(t, -1, oldIndex)
		assert.Less(t, currentIndex, oldIndex, "current member must be evaluated first")
		assert.Contains(t, operation.Query, "COALESCE(NULLIF(")
	}
	for view, found := range wantViews {
		assert.True(t, found, view)
	}

	for _, item := range migration.DownItems {
		operation, ok := item.(ModifyQueryMaterializedViewOperation)
		require.True(t, ok)
		assert.NotContains(t, operation.Query, current)
		assert.Contains(t, operation.Query, old)
	}
}
