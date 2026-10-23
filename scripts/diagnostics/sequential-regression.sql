\set ON_ERROR_STOP on
BEGIN;
SELECT count(*)=0 AS empty FROM core_events \gset
\if :empty
\else
SELECT 1/0;
\endif
INSERT INTO core_events(event_id,event_type,club_id,payload,status,created_at)
SELECT 'cleanup-fixture-'||n,'edge_edge_snapshot',(SELECT id FROM clubs LIMIT 1),'{}','processed','2026-10-09T00:00:00Z' FROM generate_series(1,6005) n;
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
DO $$
DECLARE block_no bigint:=0; block_count bigint; batch_count bigint; total_deleted bigint:=0; range_start tid; range_end tid;
BEGIN
SELECT (pg_relation_size('public.core_events')+8191)/8192 INTO block_count;
WHILE block_no<block_count LOOP
 range_start:=format('(%s,0)',block_no)::tid; range_end:=format('(%s,0)',block_no+128)::tid;
WITH batch AS MATERIALIZED (
 SELECT id,ctid AS tid FROM public.core_events
 WHERE ctid>=range_start AND ctid<range_end
 AND event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.ctid=b.tid AND e.id=b.id RETURNING e.id
)
SELECT count(*) INTO batch_count FROM deleted;
 IF batch_count=0 THEN block_no:=block_no+128; ELSE total_deleted:=total_deleted+batch_count; END IF;
END LOOP;
IF total_deleted<>6005 OR (SELECT count(*) FROM core_events)<>7 OR (SELECT count(*) FROM core_events WHERE event_id LIKE 'protected-fixture-%')<>7 THEN
 RAISE EXCEPTION 'Sequential purge regression failed';
END IF;
RAISE NOTICE 'PASS: 6005 technical fixtures deleted across physical ranges; all 7 protected cases preserved.';
END $$;
ROLLBACK;
