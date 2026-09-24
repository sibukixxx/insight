-- migration: 009_backfill_observations_analysis_id
-- purpose: attribute pre-existing observations to their analysis run where the
--          data proves it. Two paths are consulted: a pattern that cites the
--          observation, and evidence built from it. Issue #82.
-- safety: data only, no DDL. An observation is updated only when every path
--         agrees on exactly one run. Conflicting or unreachable observations
--         stay NULL (unknown) rather than being guessed.
-- rollback: forward-only.

WITH candidates AS (
    SELECT po.observation_id AS observation_id, p.analysis_id AS analysis_id
    FROM pattern_observations po
    JOIN patterns p ON p.id = po.pattern_id
    WHERE p.analysis_id IS NOT NULL
    UNION
    SELECT e.observation_id, i.analysis_id
    FROM evidence e
    JOIN insights i ON i.id = e.insight_id
    WHERE e.observation_id IS NOT NULL AND i.analysis_id IS NOT NULL
),
unambiguous AS (
    SELECT observation_id, MIN(analysis_id) AS analysis_id
    FROM candidates
    GROUP BY observation_id
    HAVING COUNT(DISTINCT analysis_id) = 1
)
UPDATE observations
SET analysis_id = (SELECT u.analysis_id FROM unambiguous u WHERE u.observation_id = observations.id)
WHERE analysis_id IS NULL
  AND id IN (SELECT observation_id FROM unambiguous);
