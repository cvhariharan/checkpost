ALTER TABLE yara_scan_matches
    DROP CONSTRAINT yara_scan_matches_count_positive,
    DROP CONSTRAINT yara_scan_matches_matches_nonempty,
    ADD COLUMN sigurl TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT yara_scan_matches_valid_result CHECK (
        (count = 0 AND length(trim(matches)) = 0)
        OR (count > 0 AND length(trim(matches)) > 0)
    );

CREATE INDEX yara_scan_matches_scan_node_idx ON yara_scan_matches (scan_id, node_id);
