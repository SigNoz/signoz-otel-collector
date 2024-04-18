# Clickhouse System Logs Receiver

Connects to clickhouse to collect logs from system tables like query_log.
Such logs are not printed to clickhouse server logs.

Collects from the query_log table only right now. Support for other system log tables like query_views_log, query_thread_log etc may be added later as needed.
