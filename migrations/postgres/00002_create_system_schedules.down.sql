DELETE FROM schedules
WHERE
    is_system_schedule = TRUE;

DELETE FROM queries
WHERE
    is_system_query = TRUE;
