import { useContext, useState } from "react"
import type { FormEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Temporal } from "@js-temporal/polyfill"
import { useForm } from "react-hook-form"
import type { MessageKey } from "~/features/i18n/messages/types"
import { useI18n } from "~/features/i18n/hooks"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { durationTextToMinutes } from "~/features/task/components/TaskForm/schema"
import { emptyInboxQuickCreateValues, inboxQuickCreateSchema, type InboxQuickCreateValues } from "./schema"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type QuickCreateType = "task" | "schedule"

function todayInTimezone(timezone: string) {
  try {
    return Temporal.Now.zonedDateTimeISO(timezone).toPlainDate().toString()
  } catch {
    return Temporal.Now.plainDateISO().toString()
  }
}

export default function InboxQuickCreate() {
  const i18n = useI18n()
  const data = useContext(InboxAPIDataContext)!
  const actions = useContext(InboxActionContext)!
  const { first } = useContext(InboxNavigationContext)!
  const [type, setType] = useState<QuickCreateType>("task")
  const [submitting, setSubmitting] = useState(false)
  const timezone = data.timezone
  const today = todayInTimezone(timezone)
  const form = useForm<InboxQuickCreateValues>({
    resolver: zodResolver(inboxQuickCreateSchema),
    defaultValues: {
      ...emptyInboxQuickCreateValues,
      formType: type,
      timezone,
      startDate: today,
      endDate: today,
    },
  })
  const { register, reset, trigger, getValues, watch, setValue, formState: { errors } } = form
  const busy = actions.busy || submitting
  const allDay = watch("allDay")
  const projectOptions = data.projects ?? []
  const titleError = errors.title?.message
    ? i18n(errors.title.message === "tasks.form.requiredTitle" ? `inbox.quickCreate.requiredTitle.${type}` : errors.title.message as MessageKey)
    : undefined
  const startDateError = errors.startDate?.message ? i18n(errors.startDate.message as MessageKey) : undefined
  const endDateError = errors.endDate?.message ? i18n(errors.endDate.message as MessageKey) : undefined
  const startTimeError = errors.startTime?.message ? i18n(errors.startTime.message as MessageKey) : undefined
  const endTimeError = errors.endTime?.message && !(errors.endDate?.message === "schedules.form.endBeforeStart" && errors.endTime.message === "schedules.form.endBeforeStart")
    ? i18n(errors.endTime.message as MessageKey)
    : undefined
  const taskEstimateError = errors.taskEstimate?.message ? i18n(errors.taskEstimate.message as MessageKey) : undefined

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busy) return
    setSubmitting(true)
    try {
      if (!await trigger(type === "task" ? ["title", "taskEstimate"] : undefined)) return
      const values = getValues()
      if (type === "task") {
        await actions.createTask({ title: values.title.trim(), projectId: values.projectId, estimatedMinutes: durationTextToMinutes(values.taskEstimate) ?? 0 })
      } else {
        await actions.createSchedule({ ...values, title: values.title.trim(), timezone })
      }
      first()
      const currentDate = todayInTimezone(timezone)
      reset({ ...emptyInboxQuickCreateValues, formType: type, timezone, startDate: currentDate, endDate: currentDate })
    } catch {
      // Action provider reports the failure; keep the user's draft in the form.
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className={`${controls.card} ${styles.card}`} aria-label={i18n("inbox.quickCreate.title")} onSubmit={(event) => void submit(event)} noValidate aria-busy={busy}>
      <div className={styles.mainRow}>
        <div className={styles.typeSwitch} role="tablist" aria-label={i18n("inbox.quickCreate.title")}>
          {(["task", "schedule"] as const).map((value) => <button
            key={value}
            className={`${styles.typeButton} ${type === value ? styles.typeButtonActive : ""} text-body-strong`}
            type="button"
            role="tab"
            aria-selected={type === value}
            onClick={() => { setType(value); setValue("formType", value, { shouldDirty: true }) }}
            disabled={busy}
          >{i18n(`inbox.quickCreate.${value}`)}</button>)}
        </div>
        <label className={styles.titleField}>
          <span className="text-label">{i18n(type === "task" ? "inbox.quickCreate.taskTitle" : "inbox.quickCreate.scheduleTitle")}</span>
          <input {...register("title")} className={`${controls.input} ${styles.titleInput}`} autoComplete="off" placeholder={i18n(type === "task" ? "inbox.quickCreate.titlePlaceholder" : "inbox.quickCreate.scheduleTitlePlaceholder")} aria-invalid={Boolean(titleError)} aria-describedby={titleError ? "inbox-title-error" : undefined} disabled={busy} />
        </label>
        <label className={styles.projectField}>
          <span className="text-label">{i18n("inbox.quickCreate.project")}</span>
          <select {...register("projectId")} className={`${controls.select} ${styles.select}`} disabled={busy}>
            <option value="">{i18n("inbox.quickCreate.noProject")}</option>
            {projectOptions.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
          </select>
        </label>
        <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} ${styles.addButton} text-button`} type="submit" disabled={busy}>
          {i18n(busy ? "inbox.quickCreate.adding" : "inbox.quickCreate.add")}
        </button>
      </div>
      {(titleError || taskEstimateError || type === "schedule" || type === "task") && <div className={styles.detailRow}>
        {type === "task" && <Field label="inbox.quickCreate.estimate" htmlFor="inbox-task-estimate" error={taskEstimateError}>
          <input {...register("taskEstimate")} className={`${controls.input} ${styles.timeInput}`} id="inbox-task-estimate" placeholder="30 / 1:30" inputMode="numeric" disabled={busy} aria-invalid={Boolean(taskEstimateError)} aria-describedby={taskEstimateError ? "inbox-task-estimate-error" : undefined} />
          <span className="text-caption">{i18n("inbox.quickCreate.estimateHint")}</span>
        </Field>}
        {type === "schedule" && <>
          <Field label="inbox.quickCreate.startDate" htmlFor="inbox-start-date" error={startDateError}>
            <input {...register("startDate")} className={`${controls.input} ${styles.dateInput}`} id="inbox-start-date" type="date" disabled={busy} aria-invalid={Boolean(startDateError)} />
          </Field>
          <Field label="inbox.quickCreate.endDate" htmlFor="inbox-end-date" error={endDateError}>
            <input {...register("endDate")} className={`${controls.input} ${styles.dateInput}`} id="inbox-end-date" type="date" disabled={busy} aria-invalid={Boolean(endDateError)} />
          </Field>
          <Field label="inbox.quickCreate.startTime" htmlFor="inbox-start-time" error={startTimeError}>
            <input {...register("startTime")} className={`${controls.input} ${styles.timeInput}`} id="inbox-start-time" type="time" disabled={busy || allDay} aria-invalid={Boolean(startTimeError)} />
          </Field>
          <Field label="inbox.quickCreate.endTime" htmlFor="inbox-end-time" error={endTimeError}>
            <input {...register("endTime")} className={`${controls.input} ${styles.timeInput}`} id="inbox-end-time" type="time" disabled={busy || allDay} aria-invalid={Boolean(endTimeError)} />
          </Field>
          <label className={`${styles.allDay} text-detail`}><input {...register("allDay")} type="checkbox" disabled={busy} />{i18n("inbox.quickCreate.allDay")}</label>
          <div className={styles.timezone}><span className="text-label">{i18n("inbox.quickCreate.timezone")}</span><span className="text-body">{timezone}</span><input {...register("timezone")} type="hidden" /></div>
        </>}
        {titleError && <p className={styles.titleError} id="inbox-title-error" role="alert">{titleError}</p>}
      </div>}
    </form>
  )
}

function Field({ label, htmlFor, error, children }: { label: MessageKey; htmlFor: string; error?: string; children: React.ReactNode }) {
  const i18n = useI18n()
  return <div className={styles.field}>
    <label className="text-label" htmlFor={htmlFor}>{i18n(label)}</label>
    {children}
    {error && <span className={styles.fieldError} id={`${htmlFor}-error`} role="alert">{error}</span>}
  </div>
}
