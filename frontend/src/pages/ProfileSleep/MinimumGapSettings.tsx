import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { getGetUsersMeQueryOptions, patchUsersMeMinimumGap } from "~/api/generated/auth"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import { minimumGapSchema, type MinimumGapValues } from "./minimumGapSchema"
import styles from "./index.module.css"

type Props = { minimumGapMinutes?: number }

export default function MinimumGapSettings({ minimumGapMinutes }: Props) {
  const i18n = useI18n()
  const client = useQueryClient()
  const form = useForm<MinimumGapValues>({
    resolver: zodResolver(minimumGapSchema),
    defaultValues: { minimumGapMinutes: minimumGapMinutes ?? 0 },
  })
  const { register, handleSubmit, reset, formState: { isDirty, isSubmitting, errors } } = form

  useEffect(() => {
    if (!isDirty && minimumGapMinutes !== undefined) reset({ minimumGapMinutes })
  }, [isDirty, minimumGapMinutes, reset])

  async function save(values: MinimumGapValues) {
    try {
      const updated = await patchUsersMeMinimumGap({ minimum_gap_minutes: values.minimumGapMinutes })
      client.setQueryData(getGetUsersMeQueryOptions().queryKey, updated)
      notify.success(i18n("weeklyPlan.toast.profileSaved"))
      reset(values)
    } catch {
      notify.error(i18n("weeklyPlan.profileGapSaveError"))
    }
  }

  return <section className={styles.section}>
    <h2 className="text-section-title">{i18n("profile.minimumGap.title")}</h2>
    <p className="text-body">{i18n("profile.minimumGap.description")}</p>
    <form className={styles.form} onSubmit={handleSubmit(save)} aria-busy={isSubmitting}>
      <label className="text-field-label" htmlFor="profile-minimum-gap">{i18n("profile.minimumGap.label")}</label>
      <input id="profile-minimum-gap" className={controls.input} type="number" min={0} max={1440} step={1} inputMode="numeric" aria-invalid={Boolean(errors.minimumGapMinutes)} aria-describedby={errors.minimumGapMinutes ? "profile-minimum-gap-error" : "profile-minimum-gap-hint"} {...register("minimumGapMinutes", { valueAsNumber: true })} />
      <p className="text-caption" id="profile-minimum-gap-hint">{i18n("profile.minimumGap.hint")}</p>
      {errors.minimumGapMinutes && <p className={styles.error} role="alert" id="profile-minimum-gap-error">{i18n("profile.minimumGap.invalid")}</p>}
      <button type="submit" className={`${controls.button} ${controls.primaryButton}`} disabled={isSubmitting || !isDirty}>{i18n("weeklyPlan.sleep.profileSave")}</button>
    </form>
  </section>
}
