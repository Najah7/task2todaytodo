import { useContext, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import type { RestProjectResponse } from "~/api/generated/projects"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useLanguage } from "~/features/i18n/hooks"
import { projectOptions2projectFormOptions } from "~/features/project/converters/projectOptions2projectFormOptions"
import { projects2projectFormValues } from "~/features/project/converters/projects2projectFormValues"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import { projectFormSchema, type ProjectFormValues } from "~/features/project/components/ProjectForm/schema"
import type { ProjectFormController } from "~/features/project/hooks/types"

export function useProjectEditForm(): ProjectFormController {
  const data = useContext(ProjectAPIDataContext)!
  const actions = useContext(ProjectActionContext)!
  const navigation = useContext(ProjectNavigationContext)!
  const [hasConflict, setHasConflict] = useState(false)
  const { language } = useLanguage()
  const initialValues = projects2projectFormValues([data.project!])[0]!
  const options = {
    types: data.options ? projectOptions2projectFormOptions(data.options.types, language) : [],
    priorities: data.options ? projectOptions2projectFormOptions(data.options.priorities, language) : [],
  }
  const form = useForm<ProjectFormValues>({
    resolver: zodResolver(projectFormSchema),
    defaultValues: initialValues,
  })
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)
  const [confirmReload, setConfirmReload] = useState(false)
  const [reloading, setReloading] = useState(false)

  async function submit(values: ProjectFormValues) {
    try {
      const result = await actions.submit(values)
      if (!result.saved && result.conflict) setHasConflict(true)
      if (!result.saved) {
        for (const [field, message] of Object.entries(result.fieldErrors ?? {})) {
          form.setError(field as keyof ProjectFormValues, { type: "server", message })
        }
        return
      }
      dirtyGuard.markClean()
      form.reset(values, { keepFieldsRef: true })
      setHasConflict(false)
      actions.saveComplete(result.revision)
      navigation.afterSave(data.project?.status)
    } catch {
      // Provider owns operational error notifications.
    }
  }

  async function reloadLatest() {
    if (!actions.reloadLatest) return
    setReloading(true)
    let latest: RestProjectResponse
    try {
      latest = await actions.reloadLatest()
    } catch {
      // Provider owns operational error notifications; keep draft and dialog intact.
      return
    } finally {
      setReloading(false)
    }
    const values = projects2projectFormValues([latest])[0]!
    form.reset(values, { keepFieldsRef: true })
    setHasConflict(false)
    setConfirmReload(false)
  }

  const viewProps: ProjectFormController["viewProps"] = {
    heading: "projects.form.editTitle",
    submitLabel: form.formState.isSubmitting ? "projects.form.submitting" : "projects.form.saveSubmit",
    options,
    onSubmit: form.handleSubmit(submit),
    onCancel: navigation.cancel,
    discardConfirmation: {
      open: dirtyGuard.dialogOpen,
      onConfirm: dirtyGuard.leave,
      onCancel: dirtyGuard.stay,
    },
    ...(hasConflict && actions.reloadLatest ? {
      conflict: {
        onRequestReload: () => setConfirmReload(true),
        reloading,
        confirmation: {
          open: confirmReload,
          onConfirm: reloadLatest,
          onCancel: () => setConfirmReload(false),
        },
      },
    } : {}),
  }

  return { form, viewProps }
}
