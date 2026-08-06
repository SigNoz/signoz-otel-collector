package schemamigrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase4MaterializedSemconvMigration(t *testing.T) {
	var migration *SchemaMigrationRecord
	for idx := range TracesMigrations {
		if TracesMigrations[idx].MigrationID == 1016 {
			migration = &TracesMigrations[idx]
			break
		}
	}
	require.NotNil(t, migration)
	wantCount := 2 * len(traceMaterializedSemconvFamilies) * 2
	require.Len(t, migration.UpItems, wantCount)
	require.Len(t, migration.DownItems, wantCount)

	for _, operation := range migration.UpItems {
		modify, ok := operation.(AlterTableModifyColumn)
		require.True(t, ok)
		assert.Contains(t, []string{"signoz_index_v3", "distributed_signoz_index_v3"}, modify.Table)
		if strings.HasSuffix(modify.Column.Name, "_exists") {
			assert.Contains(t, modify.Column.Default, " OR ")
		} else {
			assert.Contains(t, modify.Column.Default, "if(notEmpty(")
		}
		currentIndex := strings.Index(modify.Column.Default, ".name")
		if strings.Contains(modify.Column.Name, "messaging$$operation") {
			currentIndex = strings.Index(modify.Column.Default, ".type")
		}
		assert.NotEqual(t, -1, currentIndex, modify.Column.Default)
	}

	for _, operation := range migration.DownItems {
		modify, ok := operation.(AlterTableModifyColumn)
		require.True(t, ok)
		assert.NotContains(t, modify.Column.Default, " OR ")
		assert.NotContains(t, modify.Column.Default, "if(notEmpty(")
	}
}
