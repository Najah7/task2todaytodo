import { useParams } from "react-router"
import ProjectForm from "~/features/project/components/ProjectForm"
import ProjectFormBoundary from "~/features/project/components/ProjectFormBoundary"
import { useProjectEditForm } from "~/features/project/hooks/useProjectEditForm"
import { ProjectActionProvider } from "~/features/project/providers/ProjectActionProvider"
import { ProjectAPIDataProvider } from "~/features/project/providers/ProjectAPIDataProvider"
import { ProjectNavigationProvider } from "~/features/project/providers/ProjectNavigationProvider"

export default function ProjectsEditPage() {
  const { id = "" } = useParams()

  return (
    <ProjectAPIDataProvider key={id} projectId={id}>
      <ProjectActionProvider>
        <ProjectNavigationProvider>
          <ProjectFormBoundary heading="projects.form.editTitle">
            <EditProjectForm />
          </ProjectFormBoundary>
        </ProjectNavigationProvider>
      </ProjectActionProvider>
    </ProjectAPIDataProvider>
  )
}

function EditProjectForm() {
  const controller = useProjectEditForm()
  return <ProjectForm form={controller.form} {...controller.viewProps} />
}
