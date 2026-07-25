# Project Overview

task2todaytodo is a task management system that turns organized tasks into a daily execution plan.

The core problem is that planning tasks across a week or month is easy to start but hard to maintain. Interruptions create drift, and manually repairing a long-term calendar quickly becomes too much work. task2todaytodo avoids that burden by letting users manage tasks in a structured source of truth, then mechanically generating the TodoList for the day.

The product should prioritize flexible daily planning over maintaining a perfect long-term schedule.

# Core Functionality

- Manage tasks in one organized place, including projects, tasks, fixed schedules, repeatable work, and one-off work.
- Generate a TodoList as the execution plan for the day.
- Generate the TodoList from managed tasks using priority and urgency as high-level decision inputs.
- Place TaskSchedules as anchors on the day's timeline, then arrange TodoItems around the available time before, between, and after those anchors.
- Reflect TaskSchedules, which have explicit start and end times, in Google Calendar.

Avoid documenting detailed scoring formulas, recurrence calculations, carry-over mechanics, or calendar sync timing in this root file. Those details belong in feature-specific documentation or lower-level AGENT.md files.

# Domain Terms

- Inbox: The unified place where all managed tasks can be collected and reviewed.
- Project: A unit that groups Tasks and can have a goal, start date, and due date. A Task may also exist without belonging to a Project.
- Task: A container for smaller units of work. A Task can include TodoItems and TaskSchedules.
- TodoItem: A small unit of work without a fixed start and end time. TodoItems may be repeatable or one-off.
- TaskSchedule: A small unit of work with a fixed start and end time. TaskSchedules represent scheduled events and may be repeatable or one-off.
- TodoList: The execution plan generated for a specific day. It is composed of TaskSchedules and TodoItems, ordered from top to bottom as a timeline for the day.

# Repository Layout

.
├── AGENT.md: Root project context and high-level product specification.
├── backend: Backend source code and backend-specific guidance. See `backend/AGENT.md` when working there.
├── docs: Development-time documentation and reference notes.
├── frontend: Frontend source code and frontend-specific guidance. See files under `frontend/` when working there.
└── README.md: Basic project information for users and contributors.
