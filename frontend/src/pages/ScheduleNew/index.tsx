import ScheduleForm from "~/features/schedule/components/ScheduleForm"
import ScheduleFormBoundary from "~/features/schedule/components/ScheduleFormBoundary"
import { useScheduleCreateForm } from "~/features/schedule/hooks/useScheduleCreateForm"
import { ScheduleActionProvider } from "~/features/schedule/providers/ScheduleActionProvider"
import { ScheduleAPIDataProvider } from "~/features/schedule/providers/ScheduleAPIDataProvider"
import { ScheduleNavigationProvider } from "~/features/schedule/providers/ScheduleNavigationProvider"

export default function ScheduleNewPage() {
  return <ScheduleAPIDataProvider>
    <ScheduleNavigationProvider>
      <ScheduleActionProvider>
        <ScheduleFormBoundary heading="schedules.form.createTitle" loadError="schedules.form.loadError">
          <CreateScheduleForm />
        </ScheduleFormBoundary>
      </ScheduleActionProvider>
    </ScheduleNavigationProvider>
  </ScheduleAPIDataProvider>
}

function CreateScheduleForm() {
  const controller = useScheduleCreateForm()
  return <ScheduleForm form={controller.form} viewProps={controller.viewProps} />
}
