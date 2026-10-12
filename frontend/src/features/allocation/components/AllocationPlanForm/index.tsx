import type { FormEventHandler } from "react"
import { FormProvider, useFieldArray, useWatch, type FieldArrayPath, type FieldPath, type UseFormReturn } from "react-hook-form"
import { Link } from "react-router"
import { useI18n } from "~/features/i18n/hooks"
import { pausedRuleReasonMessageKey } from "~/features/allocation/lib/pausedRuleReason"
import type { RestProjectResponse } from "~/api/generated/projects"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import controls from "~/styles/controls.module.css"
import type { AllocationPlanValues } from "./schema"
import styles from "./index.module.css"

type Props = {
  form: UseFormReturn<AllocationPlanValues>
  projects: RestProjectResponse[]
  projectTitles?: Record<string, string>
  timezone: string
  busy: boolean
  onSubmit: FormEventHandler<HTMLFormElement>
  onCancel: () => void
  discardConfirmation: { open: boolean; onConfirm: () => void; onCancel: () => void }
  errors?: { rules?: string; plans?: string }
  savedMessage?: string
}

const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const

function newRule(timezone: string): AllocationPlanValues["projects"][number]["rules"][number] {
  return { weekday: "1", startTime: "09:00", endTime: "10:00", endDayOffset: "0", timezone }
}

export default function AllocationPlanForm({ form, projects, projectTitles = {}, timezone, busy, onSubmit, onCancel, discardConfirmation, errors = {}, savedMessage }: Props) {
  const i18n = useI18n()
  const { control, register, setValue, formState: { errors: validationErrors, isSubmitting } } = form
  const projectArray = useFieldArray({ control, name: "projects", keyName: "fieldKey" })
  const submitting = busy || isSubmitting
  const configured = new Set(form.getValues("projects").map((project) => project.projectId))
  const available = projects.filter((project) => project.id && !configured.has(project.id))

  function addProject(projectId: string) {
    const project = available.find((item) => item.id === projectId)
    if (!project?.id) return
    projectArray.append({ projectId: project.id, targetMinutes: 0, hasTarget: true, rules: [newRule(timezone)] })
  }

  function errorMessage(path: string) {
    const error = form.getFieldState(path as FieldPath<AllocationPlanValues>).error
    return error?.message ? i18n(error.message as never) : undefined
  }

  return <FormProvider {...form}>
    <form className={styles.form} onSubmit={onSubmit} noValidate aria-busy={submitting}>
      <div className={styles.projectList}>
        {projectArray.fields.map((project, projectIndex) => {
          const title = projects.find((option) => option.id === project.projectId)?.title || projectTitles[project.projectId] || project.projectId
          return <ProjectSlots key={project.fieldKey} control={control} register={register} setValue={setValue} projectIndex={projectIndex} projectId={project.projectId} title={title} timezone={timezone} busy={submitting} removeProject={() => projectArray.remove(projectIndex)} errorMessage={errorMessage} />
        })}
      </div>
      {available.length > 0 && <select aria-label={i18n("weeklyPlan.plan.addProject")} className={controls.select} defaultValue="" disabled={submitting} onChange={(event) => { addProject(event.currentTarget.value); event.currentTarget.value = "" }}>
        <option value="">{i18n("weeklyPlan.plan.addProject")}</option>
        {available.map((project) => project.id && <option key={project.id} value={project.id}>{project.title}</option>)}
      </select>}
      {available.length === 0 && projectArray.fields.length === 0 && <p className="text-body">{i18n("weeklyPlan.plan.noProjects")}</p>}
      {validationErrors.projects?.message && <p className={styles.error} role="alert">{i18n(validationErrors.projects.message as never)}</p>}
      {savedMessage && <p className={styles.saved} role="status">{savedMessage}</p>}
      {errors.rules && <p className={styles.error} role="alert">{i18n("weeklyPlan.plan.rulesSaveFailed", { error: errors.rules })}</p>}
      {errors.plans && <p className={styles.error} role="alert">{i18n("weeklyPlan.plan.plansSaveFailed", { error: errors.plans })}</p>}
      <footer className={styles.actions}>
        <button type="button" className={controls.button + " " + controls.neutralButton} disabled={submitting} onClick={onCancel}>{i18n("weeklyPlan.plan.cancel")}</button>
        <button type="submit" className={controls.button + " " + controls.primaryButton} disabled={submitting}>{i18n(submitting ? "weeklyPlan.plan.saving" : "weeklyPlan.plan.save")}</button>
      </footer>
      <ConfirmationDialog open={discardConfirmation.open} title={i18n("projects.dialog.discardTitle")} description={i18n("projects.dialog.discardDescription")} confirmLabel={i18n("projects.dialog.leave")} cancelLabel={i18n("projects.dialog.keepEditing")} busy={submitting} onConfirm={discardConfirmation.onConfirm} onCancel={discardConfirmation.onCancel} />
    </form>
  </FormProvider>
}

function ProjectSlots({ control, register, setValue, projectIndex, projectId, title, timezone, busy, removeProject, errorMessage }: {
  control: UseFormReturn<AllocationPlanValues>["control"]
  register: UseFormReturn<AllocationPlanValues>["register"]
  setValue: UseFormReturn<AllocationPlanValues>["setValue"]
  projectIndex: number
  projectId: string
  title: string
  timezone: string
  busy: boolean
  removeProject: () => void
  errorMessage: (path: string) => string | undefined
}) {
  const i18n = useI18n()
  const rulesName = ("projects." + projectIndex + ".rules") as FieldArrayPath<AllocationPlanValues>
  const slots = useFieldArray({ control, name: rulesName, keyName: "fieldKey" })
  const watchedRules = useWatch({ control, name: rulesName }) as AllocationPlanValues["projects"][number]["rules"] | undefined
  const targetPath = ("projects." + projectIndex + ".targetMinutes") as FieldPath<AllocationPlanValues>
  return <section className={styles.project} id={"project-" + projectId}>
    <header className={styles.projectHeader}>
      <h2 className="text-section-title">{title}</h2>
      <button type="button" className={styles.remove} disabled={busy} onClick={removeProject}>{i18n("weeklyPlan.plan.removeProject")}</button>
    </header>
    <label className="text-field-label" htmlFor={"target-" + projectId}>{i18n("weeklyPlan.plan.target")}</label>
    <input id={"target-" + projectId} className={controls.input} type="number" min={0} step={1} inputMode="numeric" disabled={busy} {...register(targetPath, { valueAsNumber: true, onChange: () => setValue(("projects." + projectIndex + ".hasTarget") as FieldPath<AllocationPlanValues>, true, { shouldDirty: true }) })} />
    {errorMessage(targetPath) && <p className={styles.error} role="alert">{errorMessage(targetPath)}</p>}
    <div className={styles.slots}>
      {slots.fields.map((slot, slotIndex) => {
        const rule = slot as AllocationPlanValues["projects"][number]["rules"][number]
        const prefix = "projects." + projectIndex + ".rules." + slotIndex + "."
        const weekdayPath = (prefix + "weekday") as FieldPath<AllocationPlanValues>
        const startPath = (prefix + "startTime") as FieldPath<AllocationPlanValues>
        const endPath = (prefix + "endTime") as FieldPath<AllocationPlanValues>
        const nextDayPath = (prefix + "endDayOffset") as FieldPath<AllocationPlanValues>
        const endDayOffset = watchedRules?.[slotIndex]?.endDayOffset
        const paused = rule.pausedReason
        return <div className={styles.slot} id={rule.id ? "rule-" + rule.id : undefined} key={slot.fieldKey}>
          <label className="text-field-label" htmlFor={"weekday-" + projectId + "-" + slotIndex}>{i18n("schedules.form.weekdays")}</label>
          <select id={"weekday-" + projectId + "-" + slotIndex} className={controls.select} disabled={busy} {...register(weekdayPath)}>
            {weekdays.map((name, day) => <option key={name} value={String(day)}>{i18n(("schedules.form.weekday." + ["sun", "mon", "tue", "wed", "thu", "fri", "sat"][day]) as never)}</option>)}
          </select>
          <label className="text-field-label" htmlFor={"start-" + projectId + "-" + slotIndex}>{i18n("weeklyPlan.plan.start")}</label>
          <input id={"start-" + projectId + "-" + slotIndex} className={controls.input} type="time" step={60} disabled={busy} {...register(startPath)} />
          <label className="text-field-label" htmlFor={"end-" + projectId + "-" + slotIndex}>{i18n("weeklyPlan.plan.end")}</label>
          <input id={"end-" + projectId + "-" + slotIndex} className={controls.input} type="time" step={60} disabled={busy} {...register(endPath)} />
          <label className={styles.nextDay}><input type="checkbox" disabled={busy} checked={endDayOffset === "1"} onChange={(event) => setValue(nextDayPath, event.currentTarget.checked ? "1" : "0", { shouldDirty: true })} />{i18n("weeklyPlan.plan.nextDay")}</label>
          {paused && <p className={styles.paused}>{i18n("weeklyPlan.plan.paused", { reason: i18n(pausedRuleReasonMessageKey(paused)) })} {rule.id && <Link to={"/weekly-plan/edit#rule-" + encodeURIComponent(rule.id)}>{i18n("weeklyPlan.plan.editRule")}</Link>}</p>}
          {(errorMessage(startPath) || errorMessage(endPath)) && <p className={styles.error} role="alert">{errorMessage(startPath) || errorMessage(endPath)}</p>}
          <button type="button" className={styles.remove} disabled={busy} onClick={() => slots.remove(slotIndex)}>{i18n("weeklyPlan.plan.removeSlot")}</button>
        </div>
      })}
    </div>
    <button type="button" className={styles.addSlot} disabled={busy} onClick={() => slots.append(newRule(timezone))}>{i18n("weeklyPlan.plan.addSlot")}</button>
  </section>
}
