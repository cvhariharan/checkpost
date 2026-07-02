ALTER TABLE policies
DROP CONSTRAINT IF EXISTS policies_severity_check;

ALTER TABLE policies
DROP COLUMN severity;
