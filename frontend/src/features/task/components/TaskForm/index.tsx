import { useState } from "react"
import { useFieldArray, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { Link } from "react-router"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { useLanguage } from "~/features/i18n/hooks"
import { messages } from "~/features/i18n/messages"
import type { TaskSelectOption } from "~/features/task/types"
import type { ActionItemFormValue, TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import PageHeading from "~/features/shared/components/PageHeading"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import { durationTextToMinutes, emptyTaskFormValues, minutesToDurationText, taskCreateSchema, type TaskFormValues } from "./schema"
import styles from "./index.module.css"

type Props = {
  mode: "create" | "edit"
  initialValues?: TaskFormValues
  estimateSource?: "manual" | "action_items"
  projectOptions: TaskSelectOption[]
  priorityOptions: TaskSelectOption[]
  returnTo?: string
  submitting?: boolean
  hasConflict?: boolean
  onCancel?: () => void
  onSubmit: (values: TaskFormValues, dirtyFields: TaskFormDirtyFields) => Promise<TaskFormSaveResult | void>
  onSaveComplete?: () => void
  onReloadLatest?: () => void
  onError?: (error: unknown) => void
}

export type TaskFormSaveResult = {
  complete: boolean
  taskSaved?: boolean
  taskResetValues?: Partial<Pick<TaskFormValues, "title" | "description" | "dueDate" | "priority" | "projectId" | "manualEstimate">>
  actionItems?: {
    index: number
    key: string
    error?: MessageKey
    fieldErrors?: Partial<Record<"title" | "estimatedMinutes" | "priority", MessageKey>>
    deletionSucceeded?: boolean
    identity?: { seriesId: string; occurrenceDate: string }
    savedFields?: Partial<Pick<ActionItemFormValue, "title" | "estimatedMinutes" | "priority">>
  }[]
  revisionRefreshError?: boolean
}

export default function TaskForm({
  mode,
  initialValues = emptyTaskFormValues,
  estimateSource,
  projectOptions,
  priorityOptions,
  returnTo = "/tasks",
  submitting = false,
  hasConflict = false,
  onCancel,
  onSubmit,
  onSaveComplete,
  onReloadLatest,
  onError,
}: Props) {
  const i18n = useI18n()
  const { language } = useLanguage()
  const edit = mode === "edit"
  const heading = edit ? "tasks.form.editTitle" : "tasks.form.createTitle"
  const { register, control, watch, handleSubmit, setError, setValue, resetField, formState: { errors, isSubmitting, dirtyFields } } = useForm<TaskFormValues>({
    resolver: zodResolver(taskCreateSchema),
    defaultValues: initialValues,
  })
  const { fields, append } = useFieldArray({ control, name: "actionItems" })
  const [actionItemErrors, setActionItemErrors] = useState<Record<string, MessageKey>>({})
  const [partialSave, setPartialSave] = useState(false)
  const [taskWasSaved, setTaskWasSaved] = useState(false)
  const [partialMessage, setPartialMessage] = useState<MessageKey>("tasks.form.partialSave")
  const busy = submitting || isSubmitting
  const watchedItems = watch("actionItems")
  const activeItems = watchedItems.filter((item) => !item.pendingDelete && !item.deletionSucceeded)
  const totalMinutes = activeItems.reduce((total, item) => total + (durationTextToMinutes(item.estimatedMinutes) ?? 0), 0)
  const hasEstimate = activeItems.some((item) => durationTextToMinutes(item.estimatedMinutes) !== undefined)
  const derivedEstimate = activeItems.length > 0 || (edit && estimateSource === "action_items")
  const taskPriority = watch("priority")

  async function submit(values: TaskFormValues) {
    try {
      const result = await onSubmit(values, dirtyFields as TaskFormDirtyFields) ?? { complete: true }
      for (const [name, value] of Object.entries(result.taskResetValues ?? {})) {
        resetField(name as "title" | "description" | "dueDate" | "priority" | "projectId" | "manualEstimate", { defaultValue: value as never })
      }
      for (const item of result.actionItems ?? []) {
        setActionItemErrors((current) => {
          const next = { ...current }
          if (item.error && !Object.keys(item.fieldErrors ?? {}).length) next[item.key] = item.error
          else delete next[item.key]
          return next
        })
        for (const [field, message] of Object.entries(item.fieldErrors ?? {})) {
          setError(`actionItems.${item.index}.${field}` as `actionItems.${number}.title`, { type: "server", message })
        }
        if (item.identity) {
          setValue(`actionItems.${item.index}.seriesId`, item.identity.seriesId, { shouldDirty: false })
          setValue(`actionItems.${item.index}.occurrenceDate`, item.identity.occurrenceDate, { shouldDirty: false })
        }
        if (item.savedFields) {
          for (const [field, value] of Object.entries(item.savedFields)) {
            resetField(`actionItems.${item.index}.${field}` as `actionItems.${number}.${"title" | "estimatedMinutes" | "priority"}`, { defaultValue: value as never })
          }
        }
        if (item.deletionSucceeded) {
          setValue(`actionItems.${item.index}.deletionSucceeded`, true, { shouldDirty: false })
        }
      }
      setPartialSave(!result.complete)
      if (result.taskSaved) setTaskWasSaved(true)
      setPartialMessage(result.revisionRefreshError ? "tasks.form.revisionRefreshError" : "tasks.form.partialSave")
      if (result.complete) onSaveComplete?.()
    } catch (cause) {
      if (cause && typeof cause === "object" && "fieldErrors" in cause) {
        const fieldErrors = (cause as { fieldErrors: Record<string, unknown> }).fieldErrors
        let unknownField = false
        for (const path of Object.keys(fieldErrors)) {
          const formPath = serverFieldPath(path)
          if (formPath) setError(formPath, { type: "server", message: "tasks.form.invalidField" })
          else unknownField = true
        }
        if (unknownField) notify.error(i18n("tasks.form.genericError"))
        return
      }
      onError?.(cause)
      if (!onError) notify.error(i18n("tasks.form.networkError"))
    }
  }

  function errorText(key?: string) {
    if (!key) return undefined
    return key in messages.ja ? i18n(key as MessageKey) : i18n("tasks.form.invalidField")
  }

  return (
    <section className={styles.page} aria-labelledby="task-form-heading">
      <nav className={styles.breadcrumb} aria-label={i18n("common.breadcrumb")}>
        <Link to="/tasks" className="text-body">{i18n("page.tasks.title")}</Link>
        <span aria-hidden="true">›</span>
        <span className="text-body" aria-current="page">{i18n(heading)}</span>
      </nav>
      <header className={styles.pageHeader}><PageHeading id="task-form-heading" messageKey={heading} /></header>
      <form className={`${controls.card} ${styles.form}`} onSubmit={handleSubmit(submit)} noValidate aria-label={i18n(heading)} aria-busy={busy}>
        {hasConflict && (
          <div className={styles.conflict} role="alert">
            <p className="text-body">{i18n("tasks.form.conflict")}</p>
            {onReloadLatest && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={onReloadLatest} disabled={busy}>{i18n("tasks.form.reloadLatest")}</button>}
          </div>
        )}
        <fieldset className={styles.fields} disabled={busy}>
          <div className={styles.wideField}>
            <label className="text-field-label" htmlFor="task-form-title">{i18n("tasks.form.title")} <span className={styles.required}>{i18n("tasks.form.titleRequired")}</span></label>
            <input {...register("title")} className={`${controls.input} ${styles.control}`} id="task-form-title" autoComplete="off" aria-invalid={Boolean(errors.title?.message)} aria-describedby={errors.title?.message ? "task-form-title-error" : undefined} />
            {errors.title?.message && <p className={styles.error} id="task-form-title-error">{errorText(errors.title.message)}</p>}
          </div>
          <div className={styles.fieldGrid}>
            <div className={styles.field}>
              <label className="text-field-label" htmlFor="task-form-project">{i18n("tasks.form.project")}</label>
              <select {...register("projectId")} className={`${controls.select} ${styles.control}`} id="task-form-project" aria-invalid={Boolean(errors.projectId?.message)} aria-describedby={errors.projectId?.message ? "task-form-project-error" : undefined}>
                <option value="">{i18n("tasks.form.noProject")}</option>
                {projectOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
              {errors.projectId?.message && <p className={styles.error} id="task-form-project-error">{errorText(errors.projectId.message)}</p>}
            </div>
            <div className={styles.field}>
              <label className="text-field-label" htmlFor="task-form-due">{i18n("tasks.form.deadline")}</label>
              <input {...register("dueDate")} className={`${controls.input} ${styles.control}`} type="date" id="task-form-due" aria-invalid={Boolean(errors.dueDate?.message)} aria-describedby={errors.dueDate?.message ? "task-form-due-error" : undefined} />
              {errors.dueDate?.message && <p className={styles.error} id="task-form-due-error">{errorText(errors.dueDate.message)}</p>}
            </div>
            <div className={styles.field}>
              <label className="text-field-label" htmlFor="task-form-priority">{i18n("tasks.form.priority")}</label>
              <select {...register("priority")} className={`${controls.select} ${styles.control}`} id="task-form-priority" aria-invalid={Boolean(errors.priority?.message)} aria-describedby={errors.priority?.message ? "task-form-priority-error" : undefined}>
                {!edit && !taskWasSaved && <option value="">{i18n("tasks.form.inheritProjectPriority")}</option>}
                {priorityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
              {errors.priority?.message && <p className={styles.error} id="task-form-priority-error">{errorText(errors.priority.message)}</p>}
            </div>
            <div className={styles.field}>
              <label className="text-field-label" htmlFor="task-form-estimate">{i18n("tasks.form.estimate")}</label>
              {derivedEstimate ? (
                <div className={`${styles.readOnly} text-body-strong`} id="task-form-estimate" aria-describedby="task-form-estimate-help">{hasEstimate ? minutesToDurationText(totalMinutes) : "—"}</div>
              ) : (
                <input {...register("manualEstimate")} className={`${controls.input} ${styles.control}`} id="task-form-estimate" placeholder="30 / 1:30" inputMode="numeric" aria-invalid={Boolean(errors.manualEstimate?.message)} aria-describedby={`task-form-estimate-help${errors.manualEstimate?.message ? " task-form-estimate-error" : ""}`} />
              )}
              <p className="text-caption" id="task-form-estimate-help">{i18n(derivedEstimate ? "tasks.form.actionItemsEstimate" : "tasks.form.manualEstimate")}</p>
              {!derivedEstimate && <p className="text-caption">{i18n("tasks.form.estimateHint")}</p>}
              {edit && estimateSource === "action_items" && <p className="text-caption">{i18n("tasks.form.estimateWillRecalculate")}</p>}
              {errorText(errors.manualEstimate?.message) && <p className={styles.error} id="task-form-estimate-error">{errorText(errors.manualEstimate?.message)}</p>}
            </div>
          </div>
          <div className={styles.wideField}>
            <label className="text-field-label" htmlFor="task-form-memo">{i18n("tasks.form.memo")}</label>
            <textarea {...register("description")} className={`${controls.input} ${styles.control} ${styles.memo}`} id="task-form-memo" placeholder={i18n("tasks.form.memoPlaceholder")} rows={3} aria-invalid={Boolean(errors.description?.message)} aria-describedby={errors.description?.message ? "task-form-memo-error" : undefined} />
            {errors.description?.message && <p className={styles.error} id="task-form-memo-error">{errorText(errors.description.message)}</p>}
          </div>
          <section className={styles.actionItems} aria-labelledby="task-action-items-heading">
            <header className={styles.actionItemsHeader}>
              <h2 className="text-body-strong" id="task-action-items-heading">{i18n("tasks.form.actionItems")}</h2>
            </header>
            <div className={styles.actionItemTable}>
              <div className={`${styles.actionItemHeader} text-label`}>
                <span>{i18n("tasks.form.actionItemTitle")}</span><span>{i18n("tasks.form.estimate")}</span><span>{i18n("tasks.form.priority")}</span><span className={styles.srOnly}>{i18n("tasks.form.removeActionItem")}</span>
              </div>
              {fields.map((field, index) => {
                const item = watchedItems[index] ?? field
                const titlePath = `actionItems.${index}.title` as const
                const occurrenceLabel = item.isRecurring && item.occurrenceDate ? ` (${formatDate(item.occurrenceDate, language)})` : ""
                const titleLabel = `${i18n("tasks.form.actionItemTitle")} ${index + 1}${occurrenceLabel}`
                const titleError = errorText(errors.actionItems?.[index]?.title?.message)
                const estimateError = errorText(errors.actionItems?.[index]?.estimatedMinutes?.message)
                const priorityError = errorText(errors.actionItems?.[index]?.priority?.message)
                const itemKey = item.clientKey ?? (item.seriesId && item.occurrenceDate ? `persisted:${item.seriesId}:${item.occurrenceDate}` : `new:${index}`)
                const saveError = actionItemErrors[itemKey]
                if (item.deletionSucceeded) return null
                if (item.pendingDelete) return (
                  <div className={styles.actionItemRow} key={field.id} role="group" aria-label={titleLabel}>
                    <p className="text-body">{item.title}</p>
                    <p className={saveError ? styles.error : "text-caption"} role={saveError ? "alert" : undefined}>{saveError ? `${item.title}: ${i18n(saveError)}` : i18n("tasks.form.actionItemPendingDelete")}</p>
                    <span />
                    <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => { setValue(`actionItems.${index}.pendingDelete`, false, { shouldDirty: true }); setActionItemErrors((current) => { const next = { ...current }; delete next[itemKey]; return next }) }}>{i18n("tasks.form.cancelDelete")}</button>
                  </div>
                )
                return (
                  <div className={styles.actionItemRow} key={field.id}>
                    <div>
                      <input {...register(titlePath)} className={`${controls.input} ${styles.control}`} aria-label={titleLabel} aria-invalid={Boolean(titleError)} aria-describedby={titleError ? `task-action-item-${index}-title-error` : undefined} />
                      {item.isRecurring && item.occurrenceDate && <p className="text-caption">{i18n("tasks.form.currentOccurrence", { date: formatDate(item.occurrenceDate, language) })}</p>}
                      {item.completed && <p className="text-caption">{i18n("tasks.form.completedCannotDelete")}</p>}
                      {titleError && <p className={styles.error} id={`task-action-item-${index}-title-error`}>{titleError}</p>}
                      {saveError && <p className={styles.error} role="alert">{i18n(saveError)}</p>}
                    </div>
                    <div>
                      <input {...register(`actionItems.${index}.estimatedMinutes`)} className={`${controls.input} ${styles.control}`} placeholder="30 / 0:30" inputMode="numeric" aria-label={`${i18n("tasks.form.estimate")} ${index + 1}`} aria-invalid={Boolean(estimateError)} aria-describedby={estimateError ? `task-action-item-${index}-estimate-error` : undefined} />
                      {estimateError && <p className={styles.error} id={`task-action-item-${index}-estimate-error`}>{estimateError}</p>}
                    </div>
                    <div className={styles.priorityField}>
                      <select {...register(`actionItems.${index}.priority`)} className={`${controls.select} ${styles.control}`} aria-label={`${i18n("tasks.form.priority")} ${index + 1}`} aria-invalid={Boolean(priorityError)} aria-describedby={priorityError ? `task-action-item-${index}-priority-error` : undefined}>
                        {!item.seriesId && <option value="">{i18n("tasks.form.inheritTaskPriority")}{taskPriority ? ` (${priorityOptions.find((option) => option.value === taskPriority)?.label ?? ""})` : ""}</option>}
                        {priorityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                      </select>
                      {priorityError && <p className={styles.error} id={`task-action-item-${index}-priority-error`}>{priorityError}</p>}
                    </div>
              <button className={`${styles.remove} ${controls.focusRing}`} type="button" aria-label={i18n("tasks.form.removeActionItemIndexed", { index: index + 1 })} onClick={() => setValue(`actionItems.${index}.pendingDelete`, true, { shouldDirty: true })} disabled={field.completed}>×</button>
                  </div>
                )
              })}
            </div>
            {partialSave && <p className={styles.error} role="alert">{i18n(!edit && taskWasSaved && partialMessage === "tasks.form.partialSave" ? "tasks.form.taskSavedActionItemsPending" : partialMessage)}</p>}
            <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} ${styles.addActionItem} text-button-small`} type="button" onClick={() => append({ clientKey: createClientKey(), title: "", estimatedMinutes: "", priority: "" })}>
              {i18n("tasks.form.addActionItem")}
            </button>
              {activeItems.length > 0 && <div className={`${styles.total} text-body-strong`}><span>{i18n("tasks.summary.total")} {i18n("tasks.form.count", { count: activeItems.length })}</span><span>{hasEstimate ? minutesToDurationText(totalMinutes) : "—"}</span></div>}
          </section>
        </fieldset>
        <footer className={styles.actions}>
          <Link className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} to={returnTo} onClick={(event) => { if (busy) event.preventDefault(); else onCancel?.() }}>{i18n("tasks.form.cancel")}</Link>
          <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={busy}>{busy ? i18n(edit ? "tasks.form.saving" : "tasks.form.submitting") : i18n(edit ? "tasks.form.save" : "tasks.form.submit")}</button>
        </footer>
      </form>
    </section>
  )
}

function createClientKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `new-${Date.now()}-${Math.random()}`
}

function serverFieldPath(path: string): "title" | "projectId" | "dueDate" | "description" | "priority" | "manualEstimate" | undefined {
  const fields = {
    title: "title",
    project_id: "projectId",
    due_date: "dueDate",
    description: "description",
    priority: "priority",
    manual_estimated_minutes: "manualEstimate",
  } as const
  if (path in fields) return fields[path as keyof typeof fields]
  return undefined
}

function formatDate(value: string, language: string): string {
  const date = new Date(`${value}T00:00:00Z`)
  return new Intl.DateTimeFormat(language === "ja" ? "ja-JP" : "en-US", { month: "short", day: "numeric", weekday: "short", timeZone: "UTC" }).format(date)
}
