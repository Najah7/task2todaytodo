import { useEffect } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { getGetUsersMeQueryOptions, patchUsersMeSleep } from "~/api/generated/auth"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { useLanguage } from "~/features/i18n/hooks"
import { allocationErrorMessage } from "~/features/allocation/lib/errorMessage"
import { fetchActiveProjects } from "~/features/project/providers/ProjectAPIDataProvider/fetchActiveProjects"
import { notify } from "~/features/shared/notification"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import MinimumGapSettings from "./MinimumGapSettings"

const schema = z.object({ wake: z.string().regex(/^\d{2}:\d{2}$/), bedtime: z.string().regex(/^\d{2}:\d{2}$/) })
type Values = z.infer<typeof schema>

export default function ProfileSleepSettings() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const client = useQueryClient()
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })
  const projectsQuery = useQuery({ queryKey: ["projects", "active-options"], queryFn: ({ signal }) => fetchActiveProjects(signal), retry: false })
  const { register, handleSubmit, reset, formState: { isDirty, isSubmitting } } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { wake: "07:00", bedtime: "23:00" },
  })
  useEffect(() => {
    if (userQuery.data && !isDirty) reset({ wake: userQuery.data.wake_up_time || "07:00", bedtime: userQuery.data.bedtime || "23:00" })
  }, [userQuery.data, isDirty, reset])

  async function save(values: Values) {
    try {
      await patchUsersMeSleep({ wake_up_time: values.wake, bedtime: values.bedtime })
      await Promise.all([
        client.invalidateQueries({ queryKey: getGetUsersMeQueryOptions().queryKey }),
        client.invalidateQueries({ predicate: (query) => typeof query.queryKey[0] === "string" && query.queryKey[0].startsWith("/api/allocation-weeks/") }),
      ])
      notify.success(i18n("weeklyPlan.toast.profileSaved"))
      reset(values)
    } catch (error) {
      notify.error(allocationErrorMessage(error, i18n, {
        status: error instanceof ApiError ? error.status : undefined,
        timezone: userQuery.data?.timezone || "UTC",
        language,
        projectNames: new Map((projectsQuery.data ?? []).filter((project) => project.id && project.title).map((project) => [project.id!, project.title!] as const)),
      }))
    }
  }

  return <>
    <PageHeading messageKey="page.profile.title" />
    <section className={styles.section}>
      <h2 className="text-section-title">{i18n("weeklyPlan.sleep.profile")}</h2>
      <p className="text-body">{i18n("weeklyPlan.sleep.profileDescription")}</p>
      {userQuery.isLoading ? <p className="text-body">{i18n("weeklyPlan.loading")}</p> : userQuery.isError ? <p role="alert">{i18n("weeklyPlan.loadError")}</p> : <form className={styles.form} onSubmit={handleSubmit(save)} aria-busy={isSubmitting}>
        <label className="text-field-label" htmlFor="profile-wake">{i18n("weeklyPlan.sleep.wake")}</label>
        <input id="profile-wake" className={controls.input} type="time" step={60} {...register("wake")} />
        <label className="text-field-label" htmlFor="profile-bedtime">{i18n("weeklyPlan.sleep.bedtime")}</label>
        <input id="profile-bedtime" className={controls.input} type="time" step={60} {...register("bedtime")} />
        <button type="submit" className={controls.button + " " + controls.primaryButton} disabled={isSubmitting || !isDirty}>{i18n("weeklyPlan.sleep.profileSave")}</button>
      </form>}
    </section>
    {!userQuery.isLoading && !userQuery.isError && userQuery.data && <MinimumGapSettings minimumGapMinutes={userQuery.data.minimum_gap_minutes ?? 0} />}
  </>
}
