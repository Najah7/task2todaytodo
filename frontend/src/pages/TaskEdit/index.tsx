import { useParams } from "react-router"
import TaskForm from "~/features/task/components/TaskForm"
import TaskFormBoundary from "~/features/task/components/TaskFormBoundary"
import { useTaskEditForm } from "~/features/task/hooks/useTaskEditForm"
import { TaskActionProvider } from "~/features/task/providers/TaskActionProvider"
import { TaskAPIDataProvider } from "~/features/task/providers/TaskAPIDataProvider"
import { TaskNavigationProvider } from "~/features/task/providers/TaskNavigationProvider"

export default function TaskEditPage() {
  const { id = "" } = useParams()
  return (
    <TaskAPIDataProvider key={id} taskId={id}>
      <TaskActionProvider>
        <TaskNavigationProvider>
          <TaskFormBoundary heading="tasks.form.editTitle" loadError="tasks.form.loadError">
            <EditTaskForm />
          </TaskFormBoundary>
        </TaskNavigationProvider>
      </TaskActionProvider>
    </TaskAPIDataProvider>
  )
}

function EditTaskForm() {
  const controller = useTaskEditForm()
  return <TaskForm form={controller.form} {...controller.viewProps} />
}
