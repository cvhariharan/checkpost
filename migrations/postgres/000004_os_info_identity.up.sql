UPDATE schedules SET
    sql = 'SELECT os.name, os.version, os.build, os.platform, oi.version AS osquery_version, si.hardware_serial FROM os_version os, osquery_info oi, system_info si;',
    description = 'Operating system name, version, platform, osquery version, and hardware serial.',
    updated_at = now()
WHERE name = 'OS Info';
