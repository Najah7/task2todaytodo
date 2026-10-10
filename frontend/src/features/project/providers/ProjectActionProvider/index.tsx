import { useContext, useEffect, useState, type ReactNode } from "react"
import { useIsMutating, useMutation, useQueryClient } from "@tanstack/react-query"
import { useSearchParams } from "react-router"
import {
  deleteProjectsId,
  getProjectsId,
  getGetProjectsIdQueryKey,
  getGetProjectsOptionsQueryKey,
  getGetProjectsQueryKey,
  patchProjectsId,
  patchProjectsIdStatus,
  postProjects,
  postProjectsIdRestore,
  getDeleteProjectsIdMutationKey,
  getPatchProjectsIdStatusMutationKey,
  getPostProjectsIdRestoreMutationKey,
  GetProjectsStatus,
} from "~/api/generated/projects"
import type { GetProjectsStatus as ProjectStatus, RestErrResponse, RestProjectListResponse, RestProjectResponse } from "~/api/generated/projects"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { projectFormValues2projectCreateRequests } from "~/features/project/converters/projectFormValues2projectCreateRequests"
import { projectFormValues2projectUpdateRequests } from "~/features/project/converters/projectFormValues2projectUpdateRequests"
import { projectListStates2apiParams } from "~/features/project/converters/projectListStates2apiParams"
import { projectSearchParams2projectListStates } from "~/features/project/converters/projectSearchParams2projectListStates"
import { getProjectErrorMessageKey, getProjectFormFieldErrors } from "~/features/project/errors"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { notify } from "~/features/shared/notification"
import { ProjectActionContext, type ProjectActions, type ProjectSubmitResult } from "./context"

export function ProjectActionProvider({ children }: { children: ReactNode }) {
  const i18n = useI18n()
  const queryClient = useQueryClient()
  const { state, projectId, project, list, projectsQuery, optionsQuery } = useContext(ProjectAPIDataContext)!
  const [searchParams] = useSearchParams()
  const listState = projectSearchParams2projectListStates([searchParams])[0]!
  const listParams = projectListStates2apiParams([listState])[0]!
  const listQueryKey = getGetProjectsQueryKey(listParams)
  const [baselineRevision, setBaselineRevision] = useState<number | undefined>(state === "ready" ? project?.revision : undefined)
  const statusMutationKey = getPatchProjectsIdStatusMutationKey()
  const trashMutationKey = getDeleteProjectsIdMutationKey()
  const restoreMutationKey = getPostProjectsIdRestoreMutationKey()

  const statusMutationCount = useIsMutating({ mutationKey: statusMutationKey })
  const trashMutationCount = useIsMutating({ mutationKey: trashMutationKey })
  const restoreMutationCount = useIsMutating({ mutationKey: restoreMutationKey })
  const busy = statusMutationCount + trashMutationCount + restoreMutationCount > 0
  const trashBusy = trashMutationCount > 0

  useEffect(() => {
    if (baselineRevision === undefined && state === "ready" && project) setBaselineRevision(project.revision)
  }, [baselineRevision, project, state])

  async function submit(values: Parameters<ProjectActions["submit"]>[0]): Promise<ProjectSubmitResult> {
    if (projectId === undefined) {
      try {
        const [request] = projectFormValues2projectCreateRequests([values])
        const created = await postProjects(request!)
        queryClient.setQueryData(getGetProjectsIdQueryKey(created.id), created)
        return { saved: true, revision: created.revision }
      } catch (error) {
        const fieldErrors = getProjectFormFieldErrors(error)
        if (fieldErrors) return { saved: false, fieldErrors }
        notify.error(i18n(getProjectErrorMessageKey(error)))
        return { saved: false }
      }
    }

    if (!project || baselineRevision === undefined) return { saved: false }
    try {
      const updated = await patchProjectsId(
        projectId,
        projectFormValues2projectUpdateRequests([values])[0]!,
        { headers: { "If-Match": `"${baselineRevision}"` } },
      )
      queryClient.setQueryData(getGetProjectsIdQueryKey(projectId), updated)
      return { saved: true, revision: updated.revision }
    } catch (error) {
      let conflict = false
      if (error instanceof ApiError) {
        const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
        conflict = error.status === 409 || details.some((detail) => detail.code === "revision_conflict")
      }
      const fieldErrors = getProjectFormFieldErrors(error)
      if (fieldErrors) return { saved: false, fieldErrors, conflict }
      notify.error(i18n(getProjectErrorMessageKey(error)))
      return { saved: false, conflict }
    }
  }

  function saveComplete(revision: number) {
    setBaselineRevision(revision)
    notify.success(i18n(projectId === undefined ? "projects.toast.created" : "projects.toast.saved"))
    void queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() })
  }

  async function retry() {
    if (list) {
      await Promise.all([
        projectsQuery.refetch(),
        ...(optionsQuery.isError ? [optionsQuery.refetch()] : []),
      ])
      return
    }
    await Promise.all([
      ...(projectId === undefined ? [] : [queryClient.refetchQueries({ queryKey: getGetProjectsIdQueryKey(projectId), type: "active" })]),
      queryClient.refetchQueries({ queryKey: getGetProjectsOptionsQueryKey(), type: "active" }),
    ])
  }

  async function reloadLatest(): Promise<RestProjectResponse> {
    if (projectId === undefined) throw new Error("Project is not in edit mode")
    try {
      const latest = await getProjectsId(projectId)
      if (!latest.can_update || latest.deleted_at !== null) throw new Error("Project is no longer editable")
      queryClient.setQueryData(getGetProjectsIdQueryKey(projectId), latest)
      setBaselineRevision(latest.revision)
      return latest
    } catch (error) {
      notify.error(i18n(getProjectErrorMessageKey(error)))
      throw error
    }
  }

  const statusMutation = useMutation({
    mutationKey: statusMutationKey,
    mutationFn: ({ target, status }: { target: RestProjectResponse; status: ProjectStatus }) => patchProjectsIdStatus(
      target.id, { status }, { headers: { "If-Match": `"${target.revision}"` } },
    ),
    onMutate: async ({ target, status }) => {
      await queryClient.cancelQueries({ queryKey: listQueryKey })
      const previous = queryClient.getQueryData<RestProjectListResponse>(listQueryKey)
      if (previous) queryClient.setQueryData(listQueryKey, removeStatusChangedProject(previous, target, status))
      return { queryKey: listQueryKey, previous }
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(context.queryKey, context.previous)
      notify.error(i18n(getProjectErrorMessageKey(error)))
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(getGetProjectsIdQueryKey(updated.id), updated)
      notify.success(i18n("projects.toast.statusChanged"))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
  })
  const trashMutation = useMutation({
    mutationKey: trashMutationKey,
    mutationFn: (target: RestProjectResponse) => deleteProjectsId(target.id, { headers: { "If-Match": `"${target.revision}"` } }),
    onError: (error) => notify.error(i18n(getProjectErrorMessageKey(error))),
    onSuccess: () => notify.success(i18n("projects.toast.trashed")),
    onSettled: (_data, _error, target) => Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetProjectsIdQueryKey(target.id) }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
    ]),
  })
  const restoreMutation = useMutation({
    mutationKey: restoreMutationKey,
    mutationFn: (target: RestProjectResponse) => postProjectsIdRestore(target.id, { headers: { "If-Match": `"${target.revision}"` } }),
    onError: (error) => notify.error(i18n(getProjectErrorMessageKey(error))),
    onSuccess: (updated) => {
      queryClient.setQueryData(getGetProjectsIdQueryKey(updated.id), updated)
      notify.success(i18n("projects.toast.restored"))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
  })

  const actions: ProjectActions = {
    busy,
    trashBusy,
    submit,
    saveComplete,
    retry,
    ...(projectId === undefined ? {} : { reloadLatest }),
    changeStatus: (target, value) => {
      if (!isProjectStatus(value) || value === target.status || busy) return
      statusMutation.mutate({ target, status: value })
    },
    trash: async (target) => {
      if (busy) return
      try {
        await trashMutation.mutateAsync(target)
      } catch {
        // The mutation reports the API error; callers can close their local dialog after this promise settles.
      }
    },
    restore: async (target) => {
      if (busy) return undefined
      try {
        return await restoreMutation.mutateAsync(target)
      } catch {
        return undefined
      }
    },
  }

  return <ProjectActionContext.Provider value={actions}>{children}</ProjectActionContext.Provider>
}

function isProjectStatus(value: string): value is ProjectStatus {
  return Object.values(GetProjectsStatus).includes(value as ProjectStatus)
}

function removeStatusChangedProject(
  previous: RestProjectListResponse,
  project: RestProjectResponse,
  nextStatus: ProjectStatus,
): RestProjectListResponse {
  const statusCounts = { ...previous.summary.status_counts }
  statusCounts[project.status] = Math.max(0, statusCounts[project.status] - 1)
  statusCounts[nextStatus] += 1
  return {
    ...previous,
    items: previous.items.filter((item) => item.id !== project.id),
    summary: {
      ...previous.summary,
      total_count: Math.max(0, previous.summary.total_count - 1),
      status_counts: statusCounts,
    },
  }
}
