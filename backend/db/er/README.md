# ER diagrams

Each diagram documents tables created by one migration pair. Migration SQL is the schema source of truth; update the matching diagram when changing a migration.

| Migration pair | Diagram |
| --- | --- |
| `000001_auth.up.sql` / `000001_auth.down.sql` | `auth.mmd` |
| `000002_project.up.sql` / `000002_project.down.sql` | `project.mmd` |
| `000003_task.up.sql` / `000003_task.down.sql` | `task.mmd` |
| `000004_schedule.up.sql` / `000004_schedule.down.sql` | `schedule.mmd` |
| `000005_todolist.up.sql` / `000005_todolist.down.sql` | `todolist.mmd` |
| `000006_admin_operations.up.sql` / `000006_admin_operations.down.sql` | `admin_operations.mmd` |
| `000007_allocation.up.sql` / `000007_allocation.down.sql` | `allocation.mmd` |

Each diagram lists every table created by its `up` migration with current columns, SQL types, nullability, primary keys, and foreign keys. Tables owned by other migration pairs appear only as key-only reference entities; comments identify their owning diagram. Composite foreign keys and unique constraints are noted in comments where needed to explain the relationships.
