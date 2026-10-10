import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { getGetProjectsIdQueryKey, getGetProjectsQueryKey, postProjects, useGetProjectsOptions } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import type { ProjectSubmitResult } from "~/features/project/types"
import ProjectForm from "~/features/project/components/ProjectForm"
import { projectFormValues2projectCreateRequests } from "~/features/project/converters/projectFormValues2projectCreateRequests"
import { projectOptions2projectFormOptions } from "~/features/project/converters/projectOptions2projectFormOptions"
import { getProjectErrorMessageKey, getProjectFormFieldErrors } from "~/features/project/errors"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ProjectsNewPage() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const optionsQuery = useGetProjectsOptions({ query: { retry: false } })
  const errorKey = optionsQuery.error ? getProjectErrorMessageKey(optionsQuery.error) : undefined

  useEffect(() => {
    if (errorKey) notify.error(i18n(errorKey))
  }, [errorKey, i18n])

  async function submit(values: ProjectFormValues): Promise<ProjectSubmitResult> {
    try {
      const [request] = projectFormValues2projectCreateRequests([values])
      const project = await postProjects(request!)
      queryClient.setQueryData(getGetProjectsIdQueryKey(project.id), project)
      return { saved: true, revision: project.revision }
    } catch (error) {
      const fieldErrors = getProjectFormFieldErrors(error)
      if (fieldErrors) return { saved: false, fieldErrors }
      throw error
    }
  }

  if (!optionsQuery.data) {
    return (
      <section className={styles.state} aria-labelledby="projects-create-heading">
        <h1 className="text-page-title" id="projects-create-heading">{i18n("projects.form.createTitle")}</h1>
        {optionsQuery.isLoading ? (
          <p className="text-body" role="status">{i18n("projects.loading")}</p>
        ) : (
          <div role="alert">
            <p className="text-body">{i18n("projects.form.loadError")}</p>
            <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => optionsQuery.refetch()}>
              {i18n("projects.retry")}
            </button>
          </div>
        )}
      </section>
    )
  }

  const types = projectOptions2projectFormOptions(optionsQuery.data.types, language)
  const priorities = projectOptions2projectFormOptions(optionsQuery.data.priorities, language)

  return (
    <ProjectForm
      mode="create"
      heading="projects.form.createTitle"
      types={types}
      priorities={priorities}
      onSubmit={submit}
      onSaveComplete={() => {
        notify.success(i18n("projects.toast.created"))
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() })
        navigate("/projects?status=open")
      }}
      onError={(error) => notify.error(i18n(getProjectErrorMessageKey(error)))}
    />
  )
}
