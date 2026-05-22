DROP INDEX IF EXISTS policy_groups_policy_idx;
DROP INDEX IF EXISTS policy_groups_group_idx;
DROP TABLE IF EXISTS policy_groups;

DROP INDEX IF EXISTS group_membership_group_idx;
DROP INDEX IF EXISTS group_membership_node_idx;
DROP TABLE IF EXISTS group_membership;

DROP INDEX IF EXISTS groups_created_at_idx;
DROP TABLE IF EXISTS groups;
