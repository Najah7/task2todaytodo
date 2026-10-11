import { useContext, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { Temporal } from "@js-temporal/polyfill"
import { useForm } from "react-hook-form"
import { ApiError } from "~/api/http"
import { getGetProjectsIdMembersQueryKey, getGetProjectsIdQueryKey, getProjectsId, getProjectsIdMembers } from "~/api/generated/projects"
import { scheduleEvents2formValues } from "~/features/schedule/converters/scheduleEvents2formValues"
import { projectMembers2assigneeOptions } from "~/features/schedule/converters/projectMembers2assigneeOptions"
import { emptyScheduleFormValues, scheduleCreateSchema, type ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import { useI18n } from "~/features/i18n/hooks"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { ScheduleAPIDataContext } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleActionContext } from "~/features/schedule/providers/ScheduleActionProvider/context"
import { ScheduleNavigationContext, type ScheduleSlotPrefill } from "~/features/schedule/providers/ScheduleNavigationProvider/context"
import type { ScheduleFormController } from "./types"

const frequencies: ScheduleFormValues["frequency"][] = ["once", "daily", "weekly", "monthly", "yearly"]

export function useScheduleCreateForm(): ScheduleFormController {
  const data = useContext(ScheduleAPIDataContext)!
  const actions = useContext(ScheduleActionContext)!
  const navigation = useContext(ScheduleNavigationContext)!
  const t = useI18n()
  const initialValues = createInitialValues(data.timezone, navigation.slotPrefill, navigation.state.projectId)
  const form = useForm<ScheduleFormValues>({ resolver: zodResolver(scheduleCreateSchema), defaultValues: initialValues })
  const selectedProjectId = form.watch("projectId")
  const membersQuery = useProjectMembers(selectedProjectId)
  const projectQuery = useProject(selectedProjectId)
  const fallbackAssigneeOptions = projectMembers2assigneeOptions(
    membersQuery.data?.items || [], projectQuery.data,
    data.userId ? { user_id: data.userId, user_name: data.userName } : undefined,
    t("schedules.projectOwner"), data.projectOptions.find((project) => project.id === selectedProjectId)?.ownerId,
    t("schedules.currentUser"),
  )
  const assigneeOptions = data.assigneeOptionsProjectId === selectedProjectId ? data.assigneeOptions : fallbackAssigneeOptions
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)
  const [serverError, setServerError] = useState<ScheduleFormController["viewProps"]["serverError"]>()
  const onProjectChange = (projectId: string) => {
    form.setValue("projectId", projectId, { shouldDirty: true, shouldValidate: true })
    form.setValue("assigneeId", "", { shouldDirty: true, shouldValidate: true })
  }

  const onSubmit = form.handleSubmit(async (values) => {
    setServerError(undefined)
    try {
      await actions.create(values)
      dirtyGuard.markClean()
      form.reset(form.getValues())
      navigation.afterSave()
    } catch {
      setServerError("schedules.error.requestFailed")
    }
  })
  const onCancel = navigation.cancel

  return {
    form,
    viewProps: {
      heading: "schedules.form.createTitle",
      submitLabel: "schedules.form.createSubmit",
      projectOptions: data.projectOptions,
      assigneeOptions,
      timezoneOptions: data.timezones,
      frequencyOptions: frequencies,
      showScopeSelector: false,
      showRecurrenceFields: true,
      recurrenceReadOnly: false,
      timezoneReadOnly: false,
      canEditContent: true,
      canAssign: true,
      onProjectChange,
      serverError,
      hasConflict: false,
      reloading: false,
      onRequestReload: () => undefined,
      reloadConfirmation: { open: false, onConfirm: () => undefined, onCancel: () => undefined },
      onSubmit,
      onCancel,
      onReloadLatest: () => undefined,
      discardConfirmation: {
        open: dirtyGuard.dialogOpen,
        onConfirm: dirtyGuard.leave,
        onCancel: dirtyGuard.stay,
      },
    },
  }
}

export function useScheduleEditForm(): ScheduleFormController {
  const data = useContext(ScheduleAPIDataContext)!
  const actions = useContext(ScheduleActionContext)!
  const navigation = useContext(ScheduleNavigationContext)!
  const event = data.event!
  const t = useI18n()
  const initialValues = scheduleEvents2formValues([event], data.blueprintRevision ? [data.blueprintRevision] : [])[0]
  const form = useForm<ScheduleFormValues>({ resolver: zodResolver(scheduleCreateSchema), defaultValues: initialValues })
  const baselineValues = useRef(initialValues)
  const selectedProjectId = form.watch("projectId")
  const membersQuery = useProjectMembers(selectedProjectId)
  const projectQuery = useProject(selectedProjectId)
  const fallbackAssigneeOptions = projectMembers2assigneeOptions(
    membersQuery.data?.items || [], projectQuery.data,
    data.userId ? { user_id: data.userId, user_name: data.userName } : undefined,
    t("schedules.projectOwner"), data.projectOptions.find((project) => project.id === selectedProjectId)?.ownerId,
    t("schedules.currentUser"),
  )
  const assigneeOptions = data.assigneeOptionsProjectId === selectedProjectId ? data.assigneeOptions : fallbackAssigneeOptions
  const dirtyGuard = useFormDraftGuard(form.formState.isDirty)
  const occurrenceRevision = useRef(event.revision)
  const blueprintRevision = useRef(data.blueprintRevisionNumber || 0)
  const [hasConflict, setHasConflict] = useState(false)
  const [confirmReload, setConfirmReload] = useState(false)
  const [reloading, setReloading] = useState(false)
  const [serverError, setServerError] = useState<ScheduleFormController["viewProps"]["serverError"]>()
  const [followingAvailable, setFollowingAvailable] = useState(
    event.permissions.canUpdateFollowing
      && !data.blueprintAccessDenied
      && Boolean(data.blueprintRevision)
      && Number.isInteger(data.blueprintRevisionNumber)
      && (data.blueprintRevisionNumber || 0) > 0,
  )
  const onProjectChange = (projectId: string) => {
    form.setValue("projectId", projectId, { shouldDirty: true, shouldValidate: true })
    form.setValue("assigneeId", "", { shouldDirty: true, shouldValidate: true })
  }

  const onSubmit = form.handleSubmit(async (values) => {
    setServerError(undefined)
    try {
      await actions.update({
        blueprintId: event.blueprintId,
        occurrenceDate: event.occurrenceDate,
        revision: occurrenceRevision.current,
        blueprintRevision: blueprintRevision.current,
        scope: values.scope,
        values,
        baselineValues: baselineValues.current,
      })
      dirtyGuard.markClean()
      form.reset(form.getValues())
      baselineValues.current = form.getValues()
      setHasConflict(false)
      navigation.afterSave()
    } catch (error) {
      if (error instanceof ApiError && (error.status === 409 || error.status === 412)) {
        setServerError("schedules.error.conflict")
        setHasConflict(true)
      } else {
        setServerError("schedules.error.requestFailed")
      }
    }
  })

  const onConfirmReload = async () => {
    setReloading(true)
    try {
      const scope = form.getValues("scope")
      const snapshot = await actions.reloadLatest(event, scope, form.getValues())
      form.reset(snapshot.values)
      baselineValues.current = snapshot.values
      occurrenceRevision.current = snapshot.occurrenceRevision
      if (snapshot.blueprintRevision !== undefined) blueprintRevision.current = snapshot.blueprintRevision
      setFollowingAvailable(snapshot.followingAvailable)
      setServerError(undefined)
      setHasConflict(false)
      setConfirmReload(false)
    } catch {
      setServerError("schedules.error.requestFailed")
    } finally {
      setReloading(false)
    }
  }
  const onCancel = navigation.cancel
  const selectedScope = form.watch("scope")
  const canEditContent = selectedScope === "current"
    ? event.permissions.canUpdate
    : event.permissions.canUpdateFollowing
  const canAssign = selectedScope === "current"
    ? event.permissions.canAssign
    : event.permissions.canUpdateFollowing
  const showRecurrenceFields = form.watch("frequency") !== "once"

  return {
    form,
    viewProps: {
      heading: "schedules.form.editTitle",
      submitLabel: "schedules.form.saveSubmit",
      projectOptions: data.projectOptions,
      assigneeOptions,
      timezoneOptions: data.timezones,
      frequencyOptions: frequencies,
      showScopeSelector: followingAvailable,
      showRecurrenceFields,
      recurrenceReadOnly: form.watch("scope") === "current",
      timezoneReadOnly: form.watch("scope") === "current",
      canEditContent,
      canAssign,
      onProjectChange,
      serverError,
      hasConflict,
      reloading: reloading || actions.busy,
      onRequestReload: () => setConfirmReload(true),
      reloadConfirmation: {
        open: confirmReload,
        onConfirm: onConfirmReload,
        onCancel: () => setConfirmReload(false),
      },
      onSubmit,
      onCancel,
      onReloadLatest: () => setConfirmReload(true),
      discardConfirmation: {
        open: dirtyGuard.dialogOpen,
        onConfirm: dirtyGuard.leave,
        onCancel: dirtyGuard.stay,
      },
    },
  }
}

function createInitialValues(timezone: string, prefill?: ScheduleSlotPrefill, projectId?: string): ScheduleFormValues {
  const today = Temporal.Now.plainDateISO(timezone || "UTC").toString()
  return {
    ...emptyScheduleFormValues,
    startDate: prefill?.startDate || today,
    endDate: prefill?.endDate || today,
    startTime: prefill?.startTime || "09:00",
    endTime: prefill?.endTime || "10:00",
    allDay: prefill?.allDay || false,
    timezone: timezone || "UTC",
    projectId: projectId || "",
    scope: "following",
  }
}

function useProjectMembers(projectId: string) {
  return useQuery({
    queryKey: getGetProjectsIdMembersQueryKey(projectId || ""),
    enabled: Boolean(projectId),
    queryFn: ({ signal }) => getProjectsIdMembers(projectId, { signal }),
  })
}

function useProject(projectId: string) {
  return useQuery({
    queryKey: getGetProjectsIdQueryKey(projectId || ""),
    enabled: Boolean(projectId),
    queryFn: ({ signal }) => getProjectsId(projectId, { signal }),
  })
}
