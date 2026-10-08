import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { getGetProjectsIdQueryKey, getGetProjectsQueryKey, postProjects, useGetProjectsOptions } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { messages } from "~/features/i18n/messages"
import type { ProjectFormOption, ProjectSubmitResult } from "~/features/project/components/ProjectForm"
import ProjectForm from "~/features/project/components/ProjectForm"
import { getProjectErrorMessageKey, getProjectFormFieldErrors } from "~/features/project/errors"
import { projectOptionLabel, toProjectCreateRequest } from "~/features/project/formData"
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
    if (errorKey) notify.error(messages[language][errorKey])
  }, [errorKey, language])

  async function submit(values: Parameters<typeof toProjectCreateRequest>[0]): Promise<ProjectSubmitResult> {
    try {
      const project = await postProjects(toProjectCreateRequest(values))
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

  const types: ProjectFormOption[] = optionsQuery.data.types.map((item) => ({ value: item.value, label: projectOptionLabel(item, language) }))
  const priorities: ProjectFormOption[] = optionsQuery.data.priorities.map((item) => ({ value: item.value, label: projectOptionLabel(item, language) }))

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
