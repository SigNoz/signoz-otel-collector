package schemamigrator

import "fmt"

type traceMaterializedSemconvFamily struct {
	column  string
	current string
	old     string
}

var traceMaterializedSemconvFamilies = []traceMaterializedSemconvFamily{
	{column: "attribute_string_db$$system", current: "db.system.name", old: "db.system"},
	{column: "attribute_string_messaging$$operation", current: "messaging.operation.type", old: "messaging.operation"},
	{column: "attribute_string_rpc$$system", current: "rpc.system.name", old: "rpc.system"},
	{column: "attribute_string_peer$$service", current: "service.peer.name", old: "peer.service"},
}

func traceMaterializedSemconvOperations(currentFirst bool) []Operation {
	tables := []string{"signoz_index_v3", "distributed_signoz_index_v3"}
	operations := make([]Operation, 0, len(tables)*len(traceMaterializedSemconvFamilies)*2)
	for _, table := range tables {
		for _, family := range traceMaterializedSemconvFamilies {
			valueDefault := fmt.Sprintf("attributes_string['%s']", family.old)
			existsDefault := fmt.Sprintf("mapContains(attributes_string, '%s')", family.old)
			if currentFirst {
				valueDefault = fmt.Sprintf(
					"if(notEmpty(attributes_string['%s']), attributes_string['%s'], attributes_string['%s'])",
					family.current,
					family.current,
					family.old,
				)
				existsDefault = fmt.Sprintf(
					"mapContains(attributes_string, '%s') OR mapContains(attributes_string, '%s')",
					family.current,
					family.old,
				)
			}
			operations = append(operations,
				AlterTableModifyColumn{
					Database: "signoz_traces",
					Table:    table,
					Column:   Column{Name: family.column, Default: valueDefault},
				},
				AlterTableModifyColumn{
					Database: "signoz_traces",
					Table:    table,
					Column:   Column{Name: family.column + "_exists", Default: existsDefault},
				},
			)
		}
	}
	return operations
}
