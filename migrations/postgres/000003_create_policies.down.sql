DROP INDEX IF EXISTS policy_membership_checked_at_idx;
DROP INDEX IF EXISTS policy_membership_policy_passes_idx;
DROP INDEX IF EXISTS policy_membership_node_idx;
DROP TABLE IF EXISTS policy_membership;

DROP INDEX IF EXISTS policies_created_at_idx;
DROP INDEX IF EXISTS policies_enabled_platform_idx;
DROP TABLE IF EXISTS policies;

DROP INDEX IF EXISTS nodes_policy_updated_at_idx;
ALTER TABLE nodes DROP COLUMN IF EXISTS policy_updated_at;
