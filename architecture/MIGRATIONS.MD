# Database Migrations

The proxy service uses a Postgres database (v15+) for storing metrics related to proxied requests. This document describes how to modify the schema of that database using migrations.

## Database Client

[bun](https://bun.uptrace.dev/) is the golang client library used for  interacting with the service's database.

## Schema Migrations Tips & Tricks

### Controlling Execution

Setting an environment variable named `RUN_DATABASE_MIGRATIONS` to true will cause the service to attempt to run migrations on startup. If unset, this value defaults to false and no migrations are run

### Migration Format

New migration files must be placed in the [migrations directory](../clients/database/postgres/migrations/), have a unique name, and start with a timestamp in the below format:

```bash
$ date '+%Y%m%d%H%M%S'
20230314095813
```

### Transactional Migrations

Each migration file will run in a single transaction, which means that you can not have for example multiple `CREATE INDEX CONCURRENTLY` statements in a given migration file. Instead break each statement out into it's own migration file.

## Troubleshooting

### Migration Table Locks

Bun (the db client) uses a simple locking mechanism while running migrations that locks an entire table by attempting to write a row `(id, table_name)` to a table called `bun_migration_locks`. The table uses `table_name` as a unique key so that only one process can lock a table at a time. After the process finishes modifying the table, it deletes its lock.

However, a problem occurs if the process dies while still holding a lock, as the lock will never be deleted & no other process will be allowed to lock the table.

If you find the migrations aren't running, check the following:

Enable logging by setting the following:
```sh
# first, make sure you're actually trying to run the migrations... :)
RUN_DATABASE_MIGRATIONS=true

# debug logging for the service logger
LOG_LEVEL=DEBUG
# enable database query logging
DATABASE_QUERY_LOGGING_ENABLED=true
```

With `LOG_LEVEL=DEBUG`, check that the service is attempting to run migrations. It will produce the following log when migrations are enabled:
```json
{
    "level": "debug",
    "time": "2023-10-17T20:46:17Z",
    "caller": "/app/service/service.go:155",
    "message": "running migrations on database"
}
```

Repeated database query logs that look like this indicate the problem is with the table locks:
```log
[bun]  20:46:58.822   INSERT                2.529ms  INSERT INTO bun_migration_locks ("id", "table_name") VALUES (DEFAULT, 'bun_migrations') RETURNING "id" 	  pgdriver.Error: ERROR: duplicate key value violates unique constraint "bun_migration_locks_table_name_key" (SQLSTATE=23505)
```

To resolve, manually connect to the database and delete the lock:
```sql
SELECT * FROM bun_migration_locks;
-- verify and locate the bad lock
| --  | id    | table_name     |
| --- | ----- | -------------- |
| --  | 12345 | bun_migrations |

-- DANGEROUS DELETE OPERATION!
-- THINK BEFORE YOU PASTE!
DELETE FROM bun_migration_locks WHERE id = 12345;
```

The service, which is retrying the lock acquisition, should be able to acquire the lock and complete the migrations (successfully removing its lock after it finishes).
