import { useParams } from "react-router"
import ScheduleForm from "~/features/schedule/components/ScheduleForm"
import ScheduleFormBoundary from "~/features/schedule/components/ScheduleFormBoundary"
import { useScheduleEditForm } from "~/features/schedule/hooks/useScheduleEditForm"
import { ScheduleActionProvider } from "~/features/schedule/providers/ScheduleActionProvider"
import { ScheduleAPIDataProvider } from "~/features/schedule/providers/ScheduleAPIDataProvider"
import { ScheduleNavigationProvider } from "~/features/schedule/providers/ScheduleNavigationProvider"

export default function ScheduleEditPage() {
  const { blueprintId = "", occurrenceDate = "" } = useParams()
  const key = `${blueprintId}:${occurrenceDate}`
  return <ScheduleAPIDataProvider key={key} blueprintId={blueprintId} occurrenceDate={occurrenceDate}>
    <ScheduleNavigationProvider>
      <ScheduleActionProvider>
        <ScheduleFormBoundary heading="schedules.form.editTitle" loadError="schedules.form.loadError">
          <EditScheduleForm />
        </ScheduleFormBoundary>
      </ScheduleActionProvider>
    </ScheduleNavigationProvider>
  </ScheduleAPIDataProvider>
}

function EditScheduleForm() {
  const controller = useScheduleEditForm()
  return <ScheduleForm form={controller.form} viewProps={controller.viewProps} />
}
