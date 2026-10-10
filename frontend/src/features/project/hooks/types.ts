import type { UseFormReturn } from "react-hook-form"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import type { ProjectFormProps } from "~/features/project/components/ProjectForm/types"

export type ProjectFormController = {
  form: UseFormReturn<ProjectFormValues>
  viewProps: ProjectFormProps
}
