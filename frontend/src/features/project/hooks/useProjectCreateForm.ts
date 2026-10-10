import { useContext } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useLanguage } from "~/features/i18n/hooks"
import { projectOptions2projectFormOptions } from "~/features/project/converters/projectOptions2projectFormOptions"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import { emptyProjectFormValues, projectFormSchema, type ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import type { ProjectFormController } from "~/features/project/hooks/types"

export function useProjectCreateForm(): ProjectFormController {
  const data = useContext(ProjectAPIDataContext)!
  const actions = useContext(ProjectActionContext)!
  const navigation = useContext(ProjectNavigationContext)!
  const { language } = useLanguage()
  const options = {
    types: data.options ? projectOptions2projectFormOptions(data.options.types, language) : [],
    priorities: data.options ? projectOptions2projectFormOptions(data.options.priorities, language) : [],
  }
  const form = useForm<ProjectFormValues>({
    resolver: zodResolver(projectFormSchema),
    defaultValues: emptyProjectFormValues,
  })
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)

  async function submit(values: ProjectFormValues) {
    try {
      const result = await actions.submit(values)
      if (!result.saved) {
        for (const [field, message] of Object.entries(result.fieldErrors ?? {})) {
          form.setError(field as keyof ProjectFormValues, { type: "server", message })
        }
        return
      }
      dirtyGuard.markClean()
      form.reset(values, { keepFieldsRef: true })
      actions.saveComplete(result.revision)
      navigation.afterSave()
    } catch {
      // Provider owns operational error notifications.
    }
  }

  const viewProps: ProjectFormController["viewProps"] = {
    heading: "projects.form.createTitle",
    submitLabel: form.formState.isSubmitting ? "projects.form.submitting" : "projects.form.createSubmit",
    options,
    onSubmit: form.handleSubmit(submit),
    onCancel: navigation.cancel,
    discardConfirmation: {
      open: dirtyGuard.dialogOpen,
      onConfirm: dirtyGuard.leave,
      onCancel: dirtyGuard.stay,
    },
  }

  return { form, viewProps }
}
