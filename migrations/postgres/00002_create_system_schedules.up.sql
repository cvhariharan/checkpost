-- Insert System Queries
INSERT INTO
    queries (title, query, description, is_system_query)
VALUES
    (
        'Disk Usage POSIX',
        'SELECT path, type, ROUND((blocks_available * blocks_size * 10e-10), 2) AS free_gb, ROUND ((blocks_available * 1.0 / blocks * 1.0) * 100, 2) AS free_perc FROM mounts WHERE path = ''/'';',
        'Retrieves disk usage information for the root mount point (POSIX)',
        TRUE
    ),
    (
        'Disk Usage Windows',
        'SELECT device_id AS path, type, (free_space * 10e-10) AS free_gb, ROUND((free_space * 1.0 / size * 1.0) * 100, 2) AS free_perc FROM logical_drives WHERE device_id = ''C:'';',
        'Retrieves disk usage information for the C: drive (Windows)',
        TRUE
    );

-- Insert System Schedules
INSERT INTO
    schedules (
        title,
        query_id_fk,
        interval,
        platform,
        snapshot,
        is_system_schedule
    )
SELECT
    'Disk Monitoring POSIX',
    id,
    3600,
    'posix',
    true,
    true
FROM
    queries
WHERE
    title = 'Disk Usage POSIX';

INSERT INTO
    schedules (
        title,
        query_id_fk,
        interval,
        platform,
        snapshot,
        is_system_schedule
    )
SELECT
    'Disk Monitoring Windows',
    id,
    3600,
    'windows',
    true,
    true
FROM
    queries
WHERE
    title = 'Disk Usage Windows';
