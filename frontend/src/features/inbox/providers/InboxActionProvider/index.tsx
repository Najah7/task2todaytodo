import { useContext, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { getGetProjectsIdTasksQueryKey, getGetTasksQueryKey, getGetTasksTaskIdActionItemsQueryKey, postProjectsIdTasks, postTasks, postTasksIdcomplete, postTasksTaskIdActionItemsIdcomplete, postTasksTaskIdActionItemsIdreopen, type RestActionItemResponse } from "~/api/generated/tasks"
import { getGetProjectsIdQueryKey, getGetProjectsQueryKey } from "~/api/generated/projects"
import { postScheduleRules, postSchedulesRuleIdOccurrenceDatecomplete } from "~/api/generated/schedules"
import { getListInboxQueryKey } from "~/api/generated/inbox"
import { getGetTodayTodoQueryKey } from "~/api/generated/todo"
import { addActionItemToToday, addTaskToToday as addTaskToday, makeActionItemUrgent, makeTaskUrgent, showAddedToTodayToast } from "~/features/todo/actions/addActionItemToToday"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { inboxQuickCreateValues2scheduleFormValues } from "~/features/inbox/converters/inboxQuickCreateValues2scheduleFormValues"
import { scheduleFormValues2ruleCreateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleCreateRequests"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import type { InboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"
import type { RestInboxItemResponse } from "~/api/generated/inbox"
import { InboxActionContext } from "./context"

type Props = { children: ReactNode }

export function InboxActionProvider({ children }: Props) {
  const contextData = useContext(InboxAPIDataContext)
  if (!contextData) throw new Error("InboxAPIDataProvider is required")
  const data = contextData
  const queryClient = useQueryClient()
  const t = useI18n()
  const [busy, setBusy] = useState(false)
  async function invalidateCommon() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
      queryClient.invalidateQueries({ queryKey: ["schedules"] }),
    ])
  }

  async function invalidateProject(projectId?: string) {
    if (!projectId) return
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetProjectsIdQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(projectId) }),
    ])
  }

  async function createTask(input: { title: string; projectId: string; estimatedMinutes: number }): Promise<void> {
    setBusy(true)
    try {
      const request = { title: input.title.trim(), manual_estimated_minutes: input.estimatedMinutes }
      if (input.projectId) await postProjectsIdTasks(input.projectId, request)
      else await postTasks(request)
      await Promise.all([invalidateCommon(), invalidateProject(input.projectId)])
      notify.success(t("tasks.toast.created"))
    } catch (error) {
      notify.error(t("tasks.form.genericError"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  async function createSchedule(values: InboxQuickCreateValues): Promise<void> {
    setBusy(true)
    try {
      const assigneeId = data.userId
      if (!assigneeId) throw new Error("Current user is not loaded")
      const scheduleValues = inboxQuickCreateValues2scheduleFormValues([values], [assigneeId])[0]!
      const request = scheduleFormValues2ruleCreateRequests([scheduleValues])[0]!
      await postScheduleRules(request)
      await Promise.all([invalidateCommon(), invalidateProject(values.projectId)])
      notify.success(t("schedules.notification.created"))
    } catch (error) {
      notify.error(t("schedules.error.requestFailed"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  async function complete(item: RestInboxItemResponse, actualMinutes?: number): Promise<void> {
    if (item.type === "task" && item.task) {
      if (!item.task.can_update) throw new Error("Task is not editable")
      setBusy(true)
      try {
        await postTasksIdcomplete(item.task.id, actualMinutes === undefined ? undefined : { actual_minutes: actualMinutes }, { headers: revisionHeaders(item.task.revision) })
        await Promise.all([invalidateCommon(), invalidateProject(item.project_id ?? undefined)])
        await queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() })
        notify.success(t("tasks.toast.updated"))
      } catch (error) {
        await Promise.all([invalidateCommon(), invalidateProject(item.project_id ?? undefined)])
        notify.error(t("tasks.form.genericError"))
        throw error
      } finally {
        setBusy(false)
      }
      return
    }
    if (item.type === "schedule" && item.schedule) {
      if (!item.schedule.can_update) throw new Error("Schedule is not editable")
      setBusy(true)
      try {
        const schedule = item.schedule
        await postSchedulesRuleIdOccurrenceDatecomplete(schedule.rule_id, schedule.occurrence_date, {
          headers: revisionHeaders(schedule.revision),
        })
        await Promise.all([invalidateCommon(), invalidateProject(item.project_id ?? undefined)])
        notify.success(t("schedules.notification.completed"))
      } catch (error) {
        await Promise.all([invalidateCommon(), invalidateProject(item.project_id ?? undefined)])
        notify.error(t("schedules.error.requestFailed"))
        throw error
      } finally {
        setBusy(false)
      }
      return
    }
    throw new Error("Inbox item has no completion payload")
  }

  async function completeActionItem(taskId: string, item: RestActionItemResponse, actualMinutes?: number): Promise<void> {
    if (!item.id) throw new Error("ActionItem has no id")
    setBusy(true)
    try {
      await postTasksTaskIdActionItemsIdcomplete(taskId, item.id, actualMinutes === undefined ? undefined : { actual_minutes: actualMinutes })
      await Promise.all([
        invalidateCommon(),
        queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
      ])
      notify.success(t("tasks.toast.actionItemUpdated"))
    } catch (error) {
      notify.error(t("tasks.form.genericError"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  async function reopenActionItem(taskId: string, item: RestActionItemResponse): Promise<void> {
    if (!item.id) throw new Error("ActionItem has no id")
    setBusy(true)
    try {
      await postTasksTaskIdActionItemsIdreopen(taskId, item.id)
      await Promise.all([
        invalidateCommon(),
        queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
      ])
      notify.success(t("tasks.toast.actionItemUpdated"))
    } catch (error) {
      notify.error(t("tasks.form.genericError"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  async function addItemToToday(taskId: string, item: RestActionItemResponse): Promise<void> {
    if (!item.id) throw new Error("ActionItem has no id")
    setBusy(true)
    try {
      const snapshot = await addActionItemToToday(item.id)
      queryClient.setQueryData(getGetTodayTodoQueryKey(), snapshot)
      await Promise.all([
        invalidateCommon(),
        queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
      ])
      showAddedToTodayToast(t, item.priority, () => {
        void makeActionItemUrgent(t, taskId, item.id!).then(() => Promise.all([
          invalidateCommon(),
          queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
        ]))
      })
    } catch {
      notify.error(t("today.error"))
    } finally {
      setBusy(false)
    }
  }

  async function addTaskToTodayAction(taskId: string, priority?: string): Promise<void> {
    setBusy(true)
    try {
      const snapshot = await addTaskToday(taskId)
      queryClient.setQueryData(getGetTodayTodoQueryKey(), snapshot)
      await Promise.all([invalidateCommon(), queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() })])
      showAddedToTodayToast(t, priority, () => {
        void makeTaskUrgent(t, taskId).then(() => Promise.all([
          invalidateCommon(),
          queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
        ]))
      })
    } catch {
      notify.error(t("today.error"))
    } finally {
      setBusy(false)
    }
  }

  async function retry() {
    await Promise.all([data.inboxQuery.refetch(), data.projectsQuery.refetch(), data.userQuery.refetch()])
  }

  async function retryOptions() {
    await Promise.all([data.projectsQuery.refetch(), data.userQuery.refetch()])
  }

  const actions = { busy, createTask, createSchedule, complete, completeActionItem, reopenActionItem, addActionItemToToday: addItemToToday, addTaskToToday: addTaskToTodayAction, retry, retryOptions }
  return <InboxActionContext.Provider value={actions}>
    {children}
  </InboxActionContext.Provider>
}

function revisionHeaders(revision: number): Record<string, string> {
  return { "If-Match": `"${revision}"` }
}
