ALTER TABLE policies
ADD COLUMN severity TEXT NOT NULL DEFAULT 'medium';

ALTER TABLE policies
ADD CONSTRAINT policies_severity_check CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info'));
