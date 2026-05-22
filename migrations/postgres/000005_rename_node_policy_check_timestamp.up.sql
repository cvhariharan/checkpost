ALTER TABLE nodes RENAME COLUMN policy_updated_at TO last_policy_check_at;
ALTER INDEX nodes_policy_updated_at_idx RENAME TO nodes_last_policy_check_at_idx;
