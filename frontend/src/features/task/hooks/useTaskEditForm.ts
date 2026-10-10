import { useContext, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useLanguage } from "~/features/i18n/hooks"
import { taskAndActionItems2formValues } from "~/features/task/converters/taskAndActionItems2formValues"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { taskCreateSchema, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormController, TaskEstimateSource } from "~/features/task/components/TaskForm/types"
import { TaskActionContext } from "~/features/task/providers/TaskActionProvider/context"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import { applyTaskFormSaveResult, setTaskServerFieldErrors } from "./applyTaskFormSaveResult"

export function useTaskEditForm(): TaskFormController {
  const data = useContext(TaskAPIDataContext)!
  const actions = useContext(TaskActionContext)!
  const navigation = useContext(TaskNavigationContext)!
  const { language } = useLanguage()
  const task = data.task!
  const initialValues = taskAndActionItems2formValues([{ task, actionItems: data.actionItemsQuery.data! }])[0]!
  const [actionItemErrors, setActionItemErrors] = useState<TaskFormController["viewProps"]["actionItemErrors"]>({})
  const [partialSaveMessage, setPartialSaveMessage] = useState<TaskFormController["viewProps"]["partialSaveMessage"]>()
  const [serverError, setServerError] = useState(false)
  const [hasConflict, setHasConflict] = useState(false)
  const [confirmReload, setConfirmReload] = useState(false)
  const [reloading, setReloading] = useState(false)
  const projects = data.projectsQuery.data ?? []
  const projectOptions = projects.map((project) => ({ value: project.id, label: project.title }))
  if (task.project_id && !projectOptions.some((option) => option.value === task.project_id)) {
    projectOptions.unshift({ value: task.project_id, label: task.project_name ?? task.project_id })
  }
  const priorityOptions = (data.optionsQuery.data?.priorities ?? []).map((priority) => ({
    value: priority.value,
    label: language === "ja" ? priority.label_jp : priority.label,
  }))
  const estimateSource: TaskEstimateSource = task.estimate_source === "action_items" ? "action_items" : "manual"
  const form = useForm<TaskFormValues>({ resolver: zodResolver(taskCreateSchema), defaultValues: initialValues })
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)

  async function submit(values: TaskFormValues) {
    setServerError(false)
    setPartialSaveMessage(undefined)
    try {
      const result = await actions.submit(values, form.formState.dirtyFields as TaskFormDirtyFields)
      applyTaskFormSaveResult(form, result, setActionItemErrors)
      if (result.revisionRefreshError) setPartialSaveMessage("tasks.form.revisionRefreshError")
      else if (!result.complete) setPartialSaveMessage("tasks.form.partialSave")
      if (!result.complete) return
      dirtyGuard.markClean()
      form.reset(form.getValues())
      setHasConflict(false)
      actions.saveComplete()
      navigation.afterSave()
    } catch (cause) {
      if (cause && typeof cause === "object" && "conflict" in cause && cause.conflict) setHasConflict(true)
      if (cause && typeof cause === "object" && "fieldErrors" in cause) {
        const fieldErrors = (cause as { fieldErrors: Record<string, unknown> }).fieldErrors
        setServerError(setTaskServerFieldErrors(form, fieldErrors))
      }
    }
  }

  async function reloadLatest() {
    if (!actions.reloadLatest) return
    setReloading(true)
    try {
      const latest = await actions.reloadLatest()
      form.reset(latest.values, { keepFieldsRef: true })
      setActionItemErrors({})
      setPartialSaveMessage(undefined)
      setServerError(false)
      setHasConflict(false)
      setConfirmReload(false)
    } catch {
      // Retain draft, conflict and confirmation when provider rejects reload.
    } finally {
      setReloading(false)
    }
  }

  const viewProps: TaskFormController["viewProps"] = {
    heading: "tasks.form.editTitle",
    submitLabel: form.formState.isSubmitting ? "tasks.form.saving" : "tasks.form.save",
    options: { projects: projectOptions, priorities: priorityOptions },
    estimateSource,
    showTaskPriorityInheritance: false,
    showActionItemPriorityInheritance: true,
    showEstimateWillRecalculate: estimateSource === "action_items",
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
    ...(hasConflict && actions.reloadLatest ? { conflict: {
      onRequestReload: () => setConfirmReload(true),
      reloading,
      confirmation: {
        open: confirmReload,
        onConfirm: reloadLatest,
        onCancel: () => setConfirmReload(false),
      },
    } } : {}),
  }
  return { form, viewProps }
}
