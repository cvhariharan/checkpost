DELETE FROM schedules WHERE name IN ('Disk Monitoring POSIX', 'Disk Monitoring Windows') AND is_system = true;
DELETE FROM queries WHERE name IN ('Disk Usage POSIX', 'Disk Usage Windows') AND is_system = true;
