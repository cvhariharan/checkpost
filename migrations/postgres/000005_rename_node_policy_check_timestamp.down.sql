ALTER INDEX nodes_last_policy_check_at_idx RENAME TO nodes_policy_updated_at_idx;
ALTER TABLE nodes RENAME COLUMN last_policy_check_at TO policy_updated_at;
