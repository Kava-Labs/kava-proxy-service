# Database Migrations

The proxy service uses a Postgres database (v15+) for storing metrics related to proxied requests. This document describes how to modify the schema of that database using migrations.

## Database Client

[bun](https://bun.uptrace.dev/) is the golang client library used for  interacting with the service's database.

## Schema Migrations Tips & Tricks

### Controlling Execution

Setting an environment variable named `RUN_DATABASE_MIGRATIONS` to true will cause the service to attempt to run migrations on startup. If unset, this value defaults to false and no migrations are run

### Migration Format

New migration files must be placed in the [migrations directory](../clients/database/migrations/), have a unique name, and start with a timestamp in the below format:

```bash
$ date '+%Y%m%d%H%M%S'
20230314095813
```

### Transactional Migrations

Each migration file will run in a single transaction, which means that you can not have for example multiple `CREATE INDEX CONCURRENTLY` statements in a given migration file. Instead break each statement out into it's own migration file.
