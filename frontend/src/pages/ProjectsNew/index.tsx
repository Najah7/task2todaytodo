import ProjectForm from "~/features/project/components/ProjectForm"
import ProjectFormBoundary from "~/features/project/components/ProjectFormBoundary"
import { useProjectCreateForm } from "~/features/project/hooks/useProjectCreateForm"
import { ProjectActionProvider } from "~/features/project/providers/ProjectActionProvider"
import { ProjectAPIDataProvider } from "~/features/project/providers/ProjectAPIDataProvider"
import { ProjectNavigationProvider } from "~/features/project/providers/ProjectNavigationProvider"

export default function ProjectsNewPage() {
  return (
    <ProjectAPIDataProvider>
      <ProjectActionProvider>
        <ProjectNavigationProvider>
          <ProjectFormBoundary heading="projects.form.createTitle">
            <CreateProjectForm />
          </ProjectFormBoundary>
        </ProjectNavigationProvider>
      </ProjectActionProvider>
    </ProjectAPIDataProvider>
  )
}

function CreateProjectForm() {
  const controller = useProjectCreateForm()
  return <ProjectForm form={controller.form} {...controller.viewProps} />
}
