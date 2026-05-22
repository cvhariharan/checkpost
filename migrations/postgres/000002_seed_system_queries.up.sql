INSERT INTO queries (name, sql, description, is_system)
VALUES
    (
        'Disk Usage POSIX',
        'SELECT path, type, ROUND((blocks_available * blocks_size * 10e-10), 2) AS free_gb, ROUND((blocks * blocks_size * 10e-10), 2) AS total_gb, ROUND ((blocks_available * 1.0 / blocks * 1.0) * 100, 2) AS free_perc FROM mounts WHERE path = ''/'';',
        'Retrieves disk usage information for the root mount point (POSIX)',
        true
    ),
    (
        'Disk Usage Windows',
        'SELECT device_id AS path, type, (free_space * 10e-10) AS free_gb, (size * 10e-10) AS total_gb, ROUND((free_space * 1.0 / size * 1.0) * 100, 2) AS free_perc FROM logical_drives WHERE device_id = ''C:'';',
        'Retrieves disk usage information for the C: drive (Windows)',
        true
    )
ON CONFLICT (name) DO UPDATE SET
    sql = EXCLUDED.sql,
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system,
    updated_at = now();

INSERT INTO schedules (name, query_id, interval_seconds, platform, snapshot, is_system)
SELECT 'Disk Monitoring POSIX', id, 3600, 'posix', true, true
FROM queries
WHERE name = 'Disk Usage POSIX'
ON CONFLICT (name) DO UPDATE SET
    query_id = EXCLUDED.query_id,
    interval_seconds = EXCLUDED.interval_seconds,
    platform = EXCLUDED.platform,
    snapshot = EXCLUDED.snapshot,
    is_system = EXCLUDED.is_system,
    updated_at = now();

INSERT INTO schedules (name, query_id, interval_seconds, platform, snapshot, is_system)
SELECT 'Disk Monitoring Windows', id, 3600, 'windows', true, true
FROM queries
WHERE name = 'Disk Usage Windows'
ON CONFLICT (name) DO UPDATE SET
    query_id = EXCLUDED.query_id,
    interval_seconds = EXCLUDED.interval_seconds,
    platform = EXCLUDED.platform,
    snapshot = EXCLUDED.snapshot,
    is_system = EXCLUDED.is_system,
    updated_at = now();
