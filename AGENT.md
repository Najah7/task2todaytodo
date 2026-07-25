# Task2TodayToDo

***Don’t live for tomorrow. Give today everything you’ve got.***

task2todaytodo is a unified task and schedule management system that turns one structured source of truth into a daily execution plan.

The core problem is that planning tasks across a week or month is easy to start but hard to maintain. Interruptions create drift, and manually repairing a long-term calendar quickly becomes too much work. task2todaytodo avoids that burden by letting users manage tasks and schedules together in a structured source of truth, then generating only today's TodoList when it is needed.

By separating long-lived task and schedule management from day-by-day execution, the product can create dynamic, flexible TodoLists for the day without forcing users to maintain a perfect long-term schedule.

# Values

- Task Management: manage all tasks in a single place, including projects, tasks, fixed schedules, repeatable work, and one-off work.
- Schedule Management: manage fixed schedules for tasks, including repeatable and one-off schedules. and also synchronize those schedules with 3rd party calendar systems like Google Calendar and ..etc.
- Generate Today's TodoList: generate a TodoList as the execution plan for the day, based on managed tasks and schedules.

# Domain Terms

- Inbox: The unified place where all managed tasks can be collected and reviewed.
- Project: A unit that groups Tasks and can have a goal, start date, and due date. A Task may also exist without belonging to a Project.
- Task: A container for smaller units of work. A Task can include TodoItems and TaskSchedules.
- TodoItem: A small unit of work without a fixed start and end time. TodoItems may be repeatable or one-off.
- TaskSchedule: A small unit of work with a fixed start and end time. TaskSchedules represent scheduled events and may be repeatable or one-off.
- TodoList: The execution plan generated for a specific day. It is composed of TaskSchedules and TodoItems, ordered from top to bottom as a timeline for the day.

# Core Relationships
- User -> Tasks (To manage tasks for a user)
- Project -> Tasks (To group tasks under a project)
- Task -> TodoItems (To break down a task into smaller units of work)
- Task -> TaskSchedules (To schedule a task at a specific time)

# Repository Layout

.
├── AGENT.md: Root project context and high-level product specification.
├── backend: Backend source code and backend-specific guidance. See `backend/AGENT.md` when working there.
├── docs: Development-time documentation and reference notes.
├── frontend: Frontend source code and frontend-specific guidance. See files under `frontend/` when working there.
└── README.md: Basic project information for users and contributors.
