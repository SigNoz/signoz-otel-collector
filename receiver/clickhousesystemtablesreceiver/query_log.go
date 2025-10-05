package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// DTO for scanning query_log rows from clickhouse
// https://clickhouse.com/docs/en/operations/system-tables/query_log
type QueryLog struct {
	// Hostname of the server executing the query.
	Hostname string `ch:"hostname"`

	// Type of an event that occurred when executing the query (QueryStart, QueryFinish, ExceptionBeforeStart, ExceptionWhileProcessing)
	EventType string `ch:"type"`

	EventDate time.Time `ch:"event_date"`
	// Event datetime
	EventTime             time.Time `ch:"event_time"`
	EventTimeMicroseconds time.Time `ch:"event_time_microseconds"`

	QueryStartTime             time.Time `ch:"query_start_time"`
	QueryStartTimeMicroseconds time.Time `ch:"query_start_time_microseconds"`
	QueryDurationMs            uint64    `ch:"query_duration_ms"`

	// Total number of rows read from all tables and table functions participated in query. It includes usual subqueries, subqueries for IN and JOIN. For distributed queries read_rows includes the total number of rows read at all replicas. Each replica sends it’s read_rows value, and the server-initiator of the query summarizes all received and local values. The cache volumes do not affect this value.
	ReadRows uint64 `ch:"read_rows"`
	// Total number of bytes read from all tables and table functions participated in query. It includes usual subqueries, subqueries for IN and JOIN. For distributed queries read_bytes includes the total number of rows read at all replicas. Each replica sends it’s read_bytes value, and the server-initiator of the query summarizes all received and local values. The cache volumes do not affect this value.
	ReadBytes uint64 `ch:"read_bytes"`

	// For INSERT queries, the number of written rows. For other queries, the column value is 0.
	WrittenRows uint64 `ch:"written_rows"`
	// For INSERT queries, the number of written bytes (uncompressed). For other queries, the column value is 0.
	WrittenBytes uint64 `ch:"written_bytes"`

	// Number of rows in a result of the SELECT query, or a number of rows in the INSERT query.
	ResultRows uint64 `ch:"result_rows"`
	// RAM volume in bytes used to store a query result.
	ResultBytes uint64 `ch:"result_bytes"`

	// Memory consumption by the query.
	MemoryUsage uint64 `ch:"memory_usage"`

	// Name of the current database.
	CurrentDatabase string `ch:"current_database"`

	// Normalized query string
	Query          string `ch:"query"`
	FormattedQuery string `ch:"formatted_query"`
	QueryKind      string `ch:"query_kind"`
	// Identical hash value without the values of literals for similar queries.
	NormalizedQueryHash string `ch:"normalized_query_hash"`

	// Names of the databases present in the query.
	Databases []string `ch:"databases"`
	// Names of the tables present in the query.
	Tables []string `ch:"tables"`
	// Names of the columns present in the query.
	Columns []string `ch:"columns"`
	// Names of the partitions present in the query.
	Partitions []string `ch:"partitions"`
	// Names of the projections used during the query execution.
	Projections []string `ch:"projections"`
	// Names of the (materialized or live) views present in the query.
	Views []string `ch:"views"`

	// Code of an exception.
	ExceptionCode int32 `ch:"exception_code"`
	// Exception message.
	Exception string `ch:"exception"`
	// Stack trace. An empty string, if the query was completed successfully.
	StackTrace string `ch:"stack_trace"`

	// Query type. Possible values: 1 — query was initiated by the client, 0 — query was initiated by another query as part of distributed query execution.',
	IsInitialQuery uint8 `ch:"is_initial_query"`

	// Name of the user who initiated the current query.
	User string `ch:"user"`

	QueryId string `ch:"query_id"`

	// IP address that was used to make the query.
	Address string `ch:"address"`
	// The client port that was used to make the query.
	Port uint16 `ch:"port"`

	// Name of the user who ran the initial query (for distributed query execution).
	InitialUser string `ch:"initial_user"`
	// ID of the initial query (for distributed query execution).
	InitialQueryId string `ch:"initial_query_id"`
	// IP address that the parent query was launched from.
	InitialAddress string `ch:"initial_address"`
	// The client port that was used to make the parent query.
	InitialPort uint16 `ch:"initial_port"`
	// Initial query starting time (for distributed query execution).
	InitialQueryStartTime time.Time `ch:"initial_query_start_time"`
	// Initial query starting time with microseconds precision (for distributed query execution).
	InitialQueryStartTimeMicroseconds time.Time `ch:"initial_query_start_time_microseconds"`

	// Interface that the query was initiated from. Possible values: 1 — TCP, 2 — HTTP.
	Interface uint8 `ch:"interface"`

	// The flag whether a query was executed over a secure interface
	IsSecure uint8 `ch:"is_secure"`

	// Operating system username who runs clickhouse-client.
	OSUser string `ch:"os_user"`

	// Hostname of the client machine where the clickhouse-client or another TCP client is run.
	ClientHostname string `ch:"client_hostname"`
	// The clickhouse-client or another TCP client name.
	ClientName string `ch:"client_name"`
	// Revision of the clickhouse-client or another TCP client.
	ClientRevision uint32 `ch:"client_revision"`
	// Major version of the clickhouse-client or another TCP client.
	ClientVersionMajor uint32 `ch:"client_version_major"`
	// Minor version of the clickhouse-client or another TCP client.
	ClientVersionMinor uint32 `ch:"client_version_minor"`
	// Patch component of the clickhouse-client or another TCP client version.
	ClientVersionPatch uint32 `ch:"client_version_patch"`

	// HTTP method that initiated the query. Possible values: 0 — The query was launched from the TCP interface, 1 — GET method was used, 2 — POST method was used.
	HttpMethod uint8 `ch:"http_method"`
	// HTTP header UserAgent passed in the HTTP query.
	HttpUserAgent string `ch:"http_user_agent"`
	// HTTP header Referer passed in the HTTP query (contains an absolute or partial address of the page making the query).
	HttpReferrer string `ch:"http_referer"`
	// HTTP header X-Forwarded-For passed in the HTTP query.
	HttpForwardedFor string `ch:"forwarded_for"`

	// The quota key specified in the quotas setting (see keyed).
	QuotaKey string `ch:"quota_key"`

	// How many times a query was forwarded between servers.
	DistributedDepth uint64 `ch:"distributed_depth"`

	// ClickHouse revision.
	ClickhouseRevision uint32 `ch:"revision"`

	// Log comment. It can be set to arbitrary string no longer than max_query_size. An empty string if it is not defined.
	LogComment string `ch:"log_comment"`

	// Thread ids that are participating in query execution. These threads may not have run simultaneously.
	ThreadIds []uint64 `ch:"thread_ids"`
	// Maximum count of simultaneous threads executing the query.
	PeakThreadsUsage uint64 `ch:"peak_threads_usage"`

	// ProfileEvents that measure different metrics. The description of them could be found in the table system.events
	ProfileEvents map[string]uint64 `ch:"ProfileEvents"`

	// Settings that were changed when the client ran the query. To enable logging changes to settings, set the log_query_settings parameter to 1.',
	Settings map[string]string `ch:"Settings"`

	// Canonical names of aggregate functions, which were used during query execution.
	UsedAggregateFunctions []string `ch:"used_aggregate_functions"`
	// Canonical names of aggregate functions combinators, which were used during query execution.',
	UsedAggregateFunctionsCombinators []string `ch:"used_aggregate_function_combinators"`
	// Canonical names of database engines, which were used during query execution.
	UsedDatabaseEngines []string `ch:"used_database_engines"`
	// Canonical names of data type families, which were used during query execution.
	UsedDataTypeFamilies []string `ch:"used_data_type_families"`
	// Canonical names of dictionaries, which were used during query execution.
	UsedDictionaries []string `ch:"used_dictionaries"`
	// Canonical names of formats, which were used during query execution.
	UsedFormats []string `ch:"used_formats"`
	// Canonical names of functions, which were used during query execution.
	UsedFunctions []string `ch:"used_functions"`
	// Canonical names of storages, which were used during query execution.
	UsedStorages []string `ch:"used_storages"`
	// Canonical names of table functions, which were used during query execution.
	UsedTableFunctions []string `ch:"used_table_functions"`
	UsedRowPolicies    []string `ch:"used_row_policies"`

	// `transaction_id` Tuple(UInt64, UInt64, UUID),
	TransactionId []any `ch:"transaction_id"`

	// Usage of the query cache during query execution. Values: \'Unknown\' = Status unknown, \'None\' = The query result was neither written into nor read from the query cache, \'Write\' = The query result was written into the query cache, \'Read\' = The query result was read from the query cache.
	QueryCacheUsage string `ch:"query_cache_usage"`

	AsyncReadCounters map[string]uint64 `ch:"asynchronous_read_counters"`
}

// Queries clickhouse `db` for query_log rows with minTs <= event_time < maxTs.
// If ClusterName is non-empty, the scrape will target `clusterAllReplicas(clusterName, system.query_log)`
func scrapeQueryLogTable(
	ctx context.Context,
	db driver.Conn,
	clusterName string,
	minTs uint32,
	maxTs uint32,
) ([]QueryLog, error) {
	tableName := "system.query_log"
	if len(clusterName) > 0 {
		tableName = fmt.Sprintf(
			"clusterAllReplicas('%s', system.query_log)", clusterName,
		)
	}

	query := fmt.Sprintf(`
		select
			hostname,
			type,
			event_date,
			event_time,
			event_time_microseconds,
			query_start_time,
			query_start_time_microseconds,
			query_duration_ms,
			read_rows,
			read_bytes,
			written_rows,
			written_bytes,
			result_rows,
			result_bytes,
			memory_usage,
			current_database,
			query,
			formatted_query,
			query_kind,
			toString(normalized_query_hash) as normalized_query_hash,
			databases,
			tables,
			columns,
			partitions,
			projections,
			views,
			exception_code,
			exception,
			stack_trace,
			is_initial_query,
			user,
			query_id,
			address,
			port,
			initial_user,
			initial_query_id,
			initial_address,
			initial_port,
			initial_query_start_time,
			initial_query_start_time_microseconds,
			interface,
			is_secure,
			os_user,
			client_hostname,
			client_name,
			client_revision,
			client_version_major,
			client_version_minor,
			client_version_patch,
			http_method,
			http_user_agent,
			http_referer,
			forwarded_for,
			quota_key,
			distributed_depth,
			revision,
			log_comment,
			thread_ids,
			peak_threads_usage,
			ProfileEvents,
			Settings,
			used_aggregate_functions,
			used_aggregate_function_combinators,
			used_database_engines,
			used_data_type_families,
			used_dictionaries,
			used_formats,
			used_functions,
			used_storages,
			used_table_functions,
			used_row_policies,
			transaction_id,
			query_cache_usage,
			asynchronous_read_counters,
		from %s
		where
			event_time >= %d
			and event_time < %d
		order by event_time asc
	`, tableName, minTs, maxTs)

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("couldn't query clickhouse query_log table: %w", err)
	}

	result := []QueryLog{}
	for rows.Next() {
		ql := QueryLog{}
		err := rows.ScanStruct(&ql)
		if err != nil {
			return nil, fmt.Errorf("couldn't scan QueryLog row into a struct: %w", err)
		}
		result = append(result, ql)
	}

	return result, rows.Err()
}

func (ql *QueryLog) toLogRecord() (plog.LogRecord, error) {
	lr := plog.NewLogRecord()

	lr.SetTimestamp(pcommon.NewTimestampFromTime(ql.EventTimeMicroseconds))

	lr.Body().SetStr(ql.Query)

	if strings.HasPrefix(ql.EventType, "Exception") {
		lr.SetSeverityNumber(plog.SeverityNumberError)
		lr.SetSeverityText("ERROR")
	} else {
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
	}

	// Populate log attributes
	qlVal := reflect.ValueOf(ql).Elem()
	qlType := qlVal.Type()
	for i := 0; i < qlVal.NumField(); i++ {
		field := qlType.Field(i)
		if attrName := field.Tag.Get("ch"); attrName != "" {
			fieldVal := qlVal.Field(i).Interface()
			// if attrName is log_comment, and it's json string, we convert it to attributes
			// with the prefix clickhouse.query_log.log_comment.<key> = value
			if attrName == "log_comment" && json.Valid([]byte(qlVal.Field(i).String())) {
				logCommentMap := map[string]any{}
				err := json.Unmarshal([]byte(qlVal.Field(i).String()), &logCommentMap)
				if err == nil {
					fieldVal = logCommentMap
				}
			}

			// prefix all the attributes with clickhouse.query_log.
			attrName = fmt.Sprintf("clickhouse.query_log.%s", attrName)
			pval := pcommonValue(fieldVal)
			// if the pval is a slice, we convert it to a string with the elements separated by commas
			// else if pval is a map, we add one attribute for each key-value pair with a prefix of the attribute name
			if pval.Type() == pcommon.ValueTypeSlice {
				ps := pval.Slice()
				elems := []string{}
				for _, pe := range ps.AsRaw() {
					elems = append(elems, fmt.Sprintf("%v", pe))
				}
				// TODO: handle error
				_ = pval.FromRaw(strings.Join(elems, ","))
				pval.CopyTo(lr.Attributes().PutEmpty(attrName))
			} else if pval.Type() == pcommon.ValueTypeMap {
				pm := pval.Map()
				pm.Range(func(k string, v pcommon.Value) bool {
					v.CopyTo(lr.Attributes().PutEmpty(fmt.Sprintf("%s.%s", attrName, k)))
					return true
				})
			} else {
				pval.CopyTo(lr.Attributes().PutEmpty(attrName))
			}
		}
	}
	lr.Attributes().PutStr("source", "clickhouse")

	return lr, nil
}

// pcommon.Value.FromRaw doesn't work for everything
func pcommonValue(v any) pcommon.Value {
	pVal := pcommon.NewValueEmpty()

	rVal := reflect.ValueOf(v)
	valueKind := rVal.Kind()

	if valueKind == reflect.Slice {
		ps := pcommon.NewSlice()
		for i := 0; i < rVal.Len(); i++ {
			pv := pcommonValue(rVal.Index(i).Interface())
			pv.CopyTo(ps.AppendEmpty())
		}
		ps.CopyTo(pVal.SetEmptySlice())

	} else if valueKind == reflect.Map {
		pmap := pcommon.NewMap()
		iter := rVal.MapRange()
		for iter.Next() {
			pv := pcommonValue(iter.Value().Interface())
			pv.CopyTo(pmap.PutEmpty(iter.Key().String()))
		}
		pmap.CopyTo(pVal.SetEmptyMap())

	} else if timeV, ok := v.(time.Time); ok {
		pVal.SetStr(timeV.Format(time.RFC3339))

	} else {
		// TODO: handle error
		_ = pVal.FromRaw(v)
	}

	return pVal
}
