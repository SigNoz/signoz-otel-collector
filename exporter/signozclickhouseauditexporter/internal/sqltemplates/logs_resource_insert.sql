INSERT INTO {{ident .Database}}.{{ident .TableName}} (
    labels,
    fingerprint,
    seen_at_ts_bucket_start
) VALUES (
    ?,
    ?,
    ?
)
