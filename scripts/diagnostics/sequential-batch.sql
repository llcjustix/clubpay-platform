WITH batch AS MATERIALIZED (
 SELECT id,ctid AS tid FROM public.core_events
 WHERE ctid>=:'start_tid'::tid AND ctid<:'end_tid'::tid
 AND event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.ctid=b.tid AND e.id=b.id RETURNING e.id
)
SELECT count(*) FROM deleted;
