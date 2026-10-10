import TaskForm from "~/features/task/components/TaskForm"
import TaskFormBoundary from "~/features/task/components/TaskFormBoundary"
import { useTaskCreateForm } from "~/features/task/hooks/useTaskCreateForm"
import { TaskActionProvider } from "~/features/task/providers/TaskActionProvider"
import { TaskAPIDataProvider } from "~/features/task/providers/TaskAPIDataProvider"
import { TaskNavigationProvider } from "~/features/task/providers/TaskNavigationProvider"

export default function TaskNewPage() {
  return (
    <TaskAPIDataProvider>
      <TaskActionProvider>
        <TaskNavigationProvider defaultReturnTo="/tasks?status=open">
          <TaskFormBoundary heading="tasks.form.createTitle" loadError="tasks.options.loadError">
            <CreateTaskForm />
          </TaskFormBoundary>
        </TaskNavigationProvider>
      </TaskActionProvider>
    </TaskAPIDataProvider>
  )
}

function CreateTaskForm() {
  const controller = useTaskCreateForm()
  return <TaskForm form={controller.form} {...controller.viewProps} />
}
