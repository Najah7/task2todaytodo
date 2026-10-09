# Demo seed

`make seed` runs `demo.sql` against the database configured in `backend/.env`.
It clears application rows in that database before inserting the sample data.
Run it only against the local development database.

The seed is repeatable and uses dates relative to the database's current date
in `Asia/Tokyo`. It creates two users so authenticated list endpoints can be
checked for user scoping. The full seed contains 20 projects, 45 tasks, 20 tags,
35 ActionItems, 25 schedules, and 15 TodoLists.

## Swagger walkthrough

Open `http://localhost:8080/swagger/index.html`.

1. Use `POST /api/login` with this request body:

   ```json
   {
     "email": "demo@example.com",
     "password": "Password1!"
   }
   ```

2. Copy the returned `token`, choose **Authorize**, and enter `Bearer <token>`.
3. Try the read endpoints below. `page_size=2` exercises cursor pagination;
   pass each response's `next_page_token` into the next request.

| Endpoint | What to inspect |
| --- | --- |
| `GET /api/users/me` | Demo user's name and `Asia/Tokyo` timezone |
| `GET /api/projects?page_size=2` | Work, study, and personal projects with different progress and priorities |
| `GET /api/projects/00000000000000000000000101/tasks?page_size=2` | Tasks scoped to the product improvement project |
| `GET /api/tasks?page_size=2` | Project and inbox tasks with different statuses and priorities |
| `GET /api/tasks/00000000000000000000000201` | Task details and assigned tags |
| `GET /api/tasks/00000000000000000000000201/action-items?page_size=2` | Ordered completed and incomplete ActionItems |
| `GET /api/tasks/00000000000000000000000204/action-items` | Weekly recurring ActionItem occurrences and frequencies |
| `GET /api/tasks/00000000000000000000000220/action-items` | ActionItem for generated task; use `page_size=1` to inspect pagination |
| `GET /api/schedules?page_size=2` | Weekly schedule series, changed occurrence, and timezone |
| `GET /api/projects/00000000000000000000000101/schedules` | Project-scoped schedules, including recurring and one-off work |
| `GET /api/schedules` | Personal schedules, filtered by assignee |
| `GET /api/task-tags?page_size=2` | User-owned tags |

List requests also accept a `fields` mask. For example, try
`fields=items(id,title,status),next_page_token` on `GET /api/tasks`.

## Demo accounts

| Email | Password |
| --- | --- |
| `demo@example.com` | `Password1!` |
| `other@example.com` | `Password1!` |

The second account has a separate project, task, and tag. Login as
`other@example.com` to inspect ownership scoping.
