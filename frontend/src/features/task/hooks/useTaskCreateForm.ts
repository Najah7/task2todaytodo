import { useContext, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useLanguage } from "~/features/i18n/hooks"
import { emptyTaskFormValues, taskCreateSchema, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormController } from "~/features/task/components/TaskForm/types"
import { TaskActionContext } from "~/features/task/providers/TaskActionProvider/context"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { applyTaskFormSaveResult, setTaskServerFieldErrors } from "./applyTaskFormSaveResult"

export function useTaskCreateForm(): TaskFormController {
  const data = useContext(TaskAPIDataContext)!
  const actions = useContext(TaskActionContext)!
  const navigation = useContext(TaskNavigationContext)!
  const { language } = useLanguage()
  const [actionItemErrors, setActionItemErrors] = useState<TaskFormController["viewProps"]["actionItemErrors"]>({})
  const [taskWasSaved, setTaskWasSaved] = useState(false)
  const [partialSaveMessage, setPartialSaveMessage] = useState<TaskFormController["viewProps"]["partialSaveMessage"]>()
  const [serverError, setServerError] = useState(false)
  const initialProjectId = navigation.state.projectId ?? ""
  const projects = data.projectsQuery.data ?? []
  const projectOptions = projects.map((project) => ({ value: project.id, label: project.title }))
  if (initialProjectId && !projectOptions.some((option) => option.value === initialProjectId)) {
    projectOptions.unshift({ value: initialProjectId, label: initialProjectId })
  }
  const priorityOptions = (data.optionsQuery.data?.priorities ?? []).map((priority) => ({
    value: priority.value,
    label: language === "ja" ? priority.label_jp : priority.label,
  }))
  const form = useForm<TaskFormValues>({
    resolver: zodResolver(taskCreateSchema),
    defaultValues: { ...emptyTaskFormValues, projectId: initialProjectId },
  })
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)

  async function submit(values: TaskFormValues) {
    setServerError(false)
    setPartialSaveMessage(undefined)
    try {
      const result = await actions.submit(values, form.formState.dirtyFields as TaskFormDirtyFields)
      applyTaskFormSaveResult(form, result, setActionItemErrors)
      if (result.taskSaved) setTaskWasSaved(true)
      if (!result.complete) {
        setPartialSaveMessage(result.revisionRefreshError
          ? "tasks.form.revisionRefreshError"
          : taskWasSaved || result.taskSaved
            ? "tasks.form.taskSavedActionItemsPending"
            : "tasks.form.partialSave")
        return
      }
      dirtyGuard.markClean()
      form.reset(form.getValues())
      actions.saveComplete()
      navigation.afterSave()
    } catch (cause) {
      if (cause && typeof cause === "object" && "fieldErrors" in cause) {
        const fieldErrors = (cause as { fieldErrors: Record<string, unknown> }).fieldErrors
        setServerError(setTaskServerFieldErrors(form, fieldErrors))
        if ("conflict" in cause && cause.conflict) setServerError(true)
        return
      }
      if (cause && typeof cause === "object" && "conflict" in cause && cause.conflict) setServerError(true)
    }
  }

  const viewProps: TaskFormController["viewProps"] = {
    heading: "tasks.form.createTitle",
    submitLabel: form.formState.isSubmitting ? "tasks.form.submitting" : "tasks.form.submit",
    options: { projects: projectOptions, priorities: priorityOptions },
    showTaskPriorityInheritance: !taskWasSaved,
    showActionItemPriorityInheritance: true,
    showEstimateWillRecalculate: false,
    partialSaveMessage,
    actionItemErrors,
    serverError,
    onClearActionItemError: (key) => setActionItemErrors((current) => {
      const next = { ...current }
      delete next[key]
      return next
    }),
    onSubmit: form.handleSubmit(submit),
    onCancel: navigation.cancel,
    discardConfirmation: {
      open: dirtyGuard.dialogOpen,
      onConfirm: dirtyGuard.leave,
      onCancel: dirtyGuard.stay,
    },
  }
  return { form, viewProps }
}
