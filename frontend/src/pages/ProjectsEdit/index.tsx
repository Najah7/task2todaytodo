import { useEffect, useState } from "react"
import type { ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useNavigate, useParams } from "react-router"
import {
  getGetProjectsIdQueryKey,
  getGetProjectsQueryKey,
  getGetProjectsIdQueryOptions,
  patchProjectsId,
  useGetProjectsId,
  useGetProjectsOptions,
} from "~/api/generated/projects"
import { ApiError } from "~/api/http"
import type { RestErrResponse, RestProjectResponse } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import ProjectForm from "~/features/project/components/ProjectForm"
import type { ProjectFormOption, ProjectFormReloadSnapshot, ProjectSubmitResult } from "~/features/project/types"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import { projectFormValues2projectUpdateRequests } from "~/features/project/converters/projectFormValues2projectUpdateRequests"
import { projectOptions2projectFormOptions } from "~/features/project/converters/projectOptions2projectFormOptions"
import { projects2projectFormValues } from "~/features/project/converters/projects2projectFormValues"
import { getProjectErrorMessageKey, getProjectFormFieldErrors } from "~/features/project/errors"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ProjectsEditPage() {
  const { id = "" } = useParams()
  const i18n = useI18n()
  const { language } = useLanguage()
  const query = useGetProjectsId(id, { query: { retry: false } })
  const optionsQuery = useGetProjectsOptions({ query: { enabled: query.isSuccess, retry: false } })
  const errorKey = query.error ? getProjectErrorMessageKey(query.error) : undefined
  const optionsErrorKey = optionsQuery.error && !query.error ? getProjectErrorMessageKey(optionsQuery.error) : undefined

  useEffect(() => {
    if (errorKey) notify.error(i18n(errorKey))
    else if (optionsErrorKey) notify.error(i18n(optionsErrorKey))
  }, [errorKey, i18n, optionsErrorKey])

  if (query.isLoading) {
    return <PageState title={i18n("projects.form.editTitle")} message={i18n("projects.loading")} status />
  }
  if (!query.data) {
    return (
      <PageState title={i18n("projects.form.editTitle")} message={i18n("projects.form.loadError")}>
        <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => query.refetch()}>
          {i18n("projects.form.reload")}
        </button>
      </PageState>
    )
  }
  if (optionsQuery.isLoading) {
    return <PageState title={i18n("projects.form.editTitle")} message={i18n("projects.loading")} status />
  }
  if (!optionsQuery.data) {
    return (
      <PageState title={i18n("projects.form.editTitle")} message={i18n("projects.form.loadError")}>
        <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => optionsQuery.refetch()}>
          {i18n("projects.retry")}
        </button>
      </PageState>
    )
  }
  if (!query.data.can_update || query.data.deleted_at !== null) {
    return <PageState title={i18n("projects.form.editTitle")} message={i18n("projects.error.forbidden")} />
  }

  const types = projectOptions2projectFormOptions(optionsQuery.data.types, language)
  const priorities = projectOptions2projectFormOptions(optionsQuery.data.priorities, language)

  return <LoadedProjectEdit key={id} project={query.data} types={types} priorities={priorities} />
}

function LoadedProjectEdit({ project, types, priorities }: { project: RestProjectResponse; types: ProjectFormOption[]; priorities: ProjectFormOption[] }) {
  const i18n = useI18n()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [baselineRevision, setBaselineRevision] = useState(project.revision)
  const [hasConflict, setHasConflict] = useState(false)
  const initialValues = projects2projectFormValues([project])[0]!

  async function submit(values: ProjectFormValues): Promise<ProjectSubmitResult> {
    try {
      const updated = await patchProjectsId(
        project.id,
        projectFormValues2projectUpdateRequests([values])[0]!,
        { headers: { "If-Match": `"${baselineRevision}"` } },
      )
      queryClient.setQueryData(getGetProjectsIdQueryKey(project.id), updated)
      return { saved: true, revision: updated.revision }
    } catch (error) {
      if (error instanceof ApiError) {
        const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
        if (error.status === 409 || details.some((detail) => detail.code === "revision_conflict")) setHasConflict(true)
      }
      const fieldErrors = getProjectFormFieldErrors(error)
      if (fieldErrors) return { saved: false, fieldErrors }
      throw error
    }
  }

  async function loadLatest(): Promise<ProjectFormReloadSnapshot> {
    const latest = await queryClient.fetchQuery(getGetProjectsIdQueryOptions(project.id))
    if (!latest.can_update || latest.deleted_at !== null) throw new Error("Project is no longer editable")
    return { values: projects2projectFormValues([latest])[0]!, revision: latest.revision }
  }

  return (
    <ProjectForm
      mode="edit"
      heading="projects.form.editTitle"
      initialValues={initialValues}
      types={types}
      priorities={priorities}
      hasConflict={hasConflict}
      onSubmit={submit}
      onSaveComplete={(revision) => {
        setBaselineRevision(revision)
        notify.success(i18n("projects.toast.saved"))
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() })
        navigate(`/projects?status=${encodeURIComponent(project.status)}`)
      }}
      onLoadLatest={loadLatest}
      onConflictResolved={(revision) => {
        setBaselineRevision(revision)
        setHasConflict(false)
      }}
      onError={(error) => notify.error(i18n(getProjectErrorMessageKey(error)))}
    />
  )
}

function PageState({ title, message, status, children }: { title: string; message: string; status?: boolean; children?: ReactNode }) {
  return (
    <section className={styles.state} aria-labelledby="project-edit-state-heading">
      <h1 className="text-page-title" id="project-edit-state-heading">{title}</h1>
      <p className="text-body" role={status ? "status" : "alert"}>{message}</p>
      {children}
    </section>
  )
}
