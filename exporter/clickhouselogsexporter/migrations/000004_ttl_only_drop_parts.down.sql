ALTER TABLE signoz_logs.logs ON CLUSTER cluster MODIFY SETTING ttl_only_drop_parts = 0;
