# ER diagrams

These diagrams live in `docs/er/`; their schema source of truth is the migration SQL in `backend/db/migrations/`. Each diagram documents tables created by one migration pair. Update the matching diagram when changing a migration.

| Migration pair | Diagram |
| --- | --- |
| [000001_auth.up.sql](../../backend/db/migrations/000001_auth.up.sql) / [000001_auth.down.sql](../../backend/db/migrations/000001_auth.down.sql) | [auth.mmd](auth.mmd) |
| [000002_project.up.sql](../../backend/db/migrations/000002_project.up.sql) / [000002_project.down.sql](../../backend/db/migrations/000002_project.down.sql) | [project.mmd](project.mmd) |
| [000003_task.up.sql](../../backend/db/migrations/000003_task.up.sql) / [000003_task.down.sql](../../backend/db/migrations/000003_task.down.sql) | [task.mmd](task.mmd) |
| [000004_schedule.up.sql](../../backend/db/migrations/000004_schedule.up.sql) / [000004_schedule.down.sql](../../backend/db/migrations/000004_schedule.down.sql) | [schedule.mmd](schedule.mmd) |
| [000005_todo.up.sql](../../backend/db/migrations/000005_todo.up.sql) / [000005_todo.down.sql](../../backend/db/migrations/000005_todo.down.sql) | [todo.mmd](todo.mmd) |
| [000006_admin_operations.up.sql](../../backend/db/migrations/000006_admin_operations.up.sql) / [000006_admin_operations.down.sql](../../backend/db/migrations/000006_admin_operations.down.sql) | [admin_operations.mmd](admin_operations.mmd) |
| [000007_allocation.up.sql](../../backend/db/migrations/000007_allocation.up.sql) / [000007_allocation.down.sql](../../backend/db/migrations/000007_allocation.down.sql) | [allocation.mmd](allocation.mmd) |

Each diagram lists every table created by its `up` migration with current columns, SQL types, nullability, primary keys, and foreign keys. Tables owned by other migration pairs appear only as key-only reference entities; comments identify their owning diagram. Composite foreign keys and unique constraints are noted in comments where needed to explain the relationships.
