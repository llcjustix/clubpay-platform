\set ON_ERROR_STOP on
BEGIN;
SELECT count(*)=0 AS empty FROM core_events \gset
\if :empty
\else
SELECT 1/0;
\endif
INSERT INTO core_events(event_id,event_type,club_id,payload,status,created_at)
SELECT 'cleanup-fixture-'||n,'edge_edge_snapshot',(SELECT id FROM clubs LIMIT 1),'{}','processed','2026-10-09T00:00:00Z' FROM generate_series(1,1005) n;
INSERT INTO core_events(event_id,event_type,club_id,payload,status,created_at,grant_id,core_session_id,external_pc_id)
SELECT 'protected-fixture-'||n,
 CASE WHEN n=1 THEN 'session_started' ELSE 'edge_edge_snapshot' END,
 CASE WHEN n=2 THEN NULL ELSE (SELECT id FROM clubs LIMIT 1) END,
 '{}',CASE WHEN n=3 THEN 'failed' ELSE 'processed' END,
 CASE WHEN n=4 THEN timestamptz '2026-10-11T00:00:00Z' ELSE timestamptz '2026-10-09T00:00:00Z' END,
 CASE WHEN n=5 THEN gen_random_uuid() END,
 CASE WHEN n=6 THEN 'protected-session' END,
 CASE WHEN n=7 THEN 'protected-pc' END
FROM generate_series(1,7) n;
\set after ''
WITH batch AS MATERIALIZED (
 SELECT id FROM public.core_events
 WHERE event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 AND (NULLIF(:'after','') IS NULL OR id>NULLIF(:'after','')::uuid)
 ORDER BY id LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.id=b.id RETURNING e.id
)
SELECT count(*),COALESCE((SELECT max(id::text) FROM batch),'') FROM deleted \gset
SELECT 1 / ((:'count'::int=1000)::int) AS expected_batch_verified;
\set after :coalesce
WITH batch AS MATERIALIZED (
 SELECT id FROM public.core_events
 WHERE event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 AND (NULLIF(:'after','') IS NULL OR id>NULLIF(:'after','')::uuid)
 ORDER BY id LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.id=b.id RETURNING e.id
)
SELECT count(*),COALESCE((SELECT max(id::text) FROM batch),'') FROM deleted \gset
SELECT 1 / ((:'count'::int=5)::int) AS expected_batch_verified;
\set after :coalesce
WITH batch AS MATERIALIZED (
 SELECT id FROM public.core_events
 WHERE event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 AND (NULLIF(:'after','') IS NULL OR id>NULLIF(:'after','')::uuid)
 ORDER BY id LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.id=b.id RETURNING e.id
)
SELECT count(*),COALESCE((SELECT max(id::text) FROM batch),'') FROM deleted \gset
SELECT 1 / ((:'count'::int=0)::int) AS expected_batch_verified;

SELECT 1 / ((count(*)=7)::int) AS protected_rows_preserved FROM core_events;
SELECT 1 / ((count(*)=7)::int) AS protected_ids_preserved FROM core_events WHERE event_id LIKE 'protected-fixture-%';
ROLLBACK;
