DELETE FROM yara_scan_matches WHERE count = 0;
DROP INDEX yara_scan_matches_scan_node_idx;
ALTER TABLE yara_scan_matches
    DROP CONSTRAINT yara_scan_matches_valid_result,
    DROP COLUMN sigurl,
    ADD CONSTRAINT yara_scan_matches_count_positive CHECK (count > 0),
    ADD CONSTRAINT yara_scan_matches_matches_nonempty CHECK (length(trim(matches)) > 0);
