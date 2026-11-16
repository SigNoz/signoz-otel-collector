package sqltemplates

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

var templateFuncs = template.FuncMap{
	"ident": func(s string) string {
		return fmt.Sprintf("`%s`", strings.ReplaceAll(s, "`", "\\`"))
	},
}

func newTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(templateFuncs).Parse(text))
}

//go:embed logs_insert.sql
var LogsInsert string

//go:embed logs_resource_insert.sql
var LogsResourceInsert string

var (
	LogsInsertTmpl         = newTemplate("logs_insert", LogsInsert)
	LogsResourceInsertTmpl = newTemplate("logs_resource_insert", LogsResourceInsert)
)

type InsertData struct {
	Database  string
	TableName string
}
