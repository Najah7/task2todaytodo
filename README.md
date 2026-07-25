# Task2TodayToDo

***Don’t live for tomorrow. Give today everything you’ve got.***

task2todaytodo is a unified task and schedule management system that turns one structured source of truth into a daily execution plan.

The core problem is that planning tasks across a week or month is easy to start but hard to maintain. Interruptions create drift, and manually repairing a long-term calendar quickly becomes too much work. task2todaytodo avoids that burden by letting users manage tasks and schedules together in a structured source of truth, then generating only today's TodoList when it is needed.

By separating long-lived task and schedule management from day-by-day execution, the product can create dynamic, flexible TodoLists for the day without forcing users to maintain a perfect long-term schedule.

# Values

- Task Management: manage all tasks in a single place, including projects, tasks, fixed schedules, repeatable work, and one-off work.
- Schedule Management: manage fixed schedules for tasks, including repeatable and one-off schedules. and also synchronize those schedules with 3rd party calendar systems like Google Calendar and ..etc.
- Generate Today's TodoList: generate a TodoList as the execution plan for the day, based on managed tasks and schedules.

## Backend quick start

Prerequisites: Docker, Go, Air.

```bash
cd backend
cp .env.example .env # first setup only
make env-up
make migrate-up
air
```

API docs: <http://localhost:8080/swagger/index.html>

Check Useful backend commands:

```bash
make help        # show help
```
