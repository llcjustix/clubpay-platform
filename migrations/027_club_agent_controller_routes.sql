-- Controller addresses are shared club settings, not browser preferences.
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS agent_primary_controller_url TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS agent_fallback_controller_url TEXT;

-- Earlier installers already stored the pair per PC. Recover it only when
-- every configured, non-deleted PC agrees; never pick an arbitrary PC's pair.
WITH agreed AS (
  SELECT club_id, min(agent_primary_controller_url) AS primary_url,
         min(COALESCE(agent_fallback_controller_url, '')) AS fallback_url
  FROM pc_refs
  WHERE status_cache <> 'deleted' AND COALESCE(agent_primary_controller_url, '') <> ''
  GROUP BY club_id
  HAVING count(DISTINCT (agent_primary_controller_url, COALESCE(agent_fallback_controller_url, ''))) = 1
)
UPDATE clubs c
SET agent_primary_controller_url = a.primary_url,
    agent_fallback_controller_url = NULLIF(a.fallback_url, '')
FROM agreed a
WHERE c.id = a.club_id
  AND COALESCE(c.agent_primary_controller_url, '') = ''
  AND COALESCE(c.agent_fallback_controller_url, '') = '';
