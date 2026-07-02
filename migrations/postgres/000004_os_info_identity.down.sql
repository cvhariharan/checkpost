UPDATE schedules SET
    sql = 'SELECT name, version, build, platform FROM os_version;',
    description = 'Operating system name, version, and platform.',
    updated_at = now()
WHERE name = 'OS Info';
