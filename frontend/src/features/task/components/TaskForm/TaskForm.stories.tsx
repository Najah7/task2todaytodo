import { useState } from "react"
import { useForm } from "react-hook-form"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { createMemoryRouter, RouterProvider } from "react-router"
import App from "~/App"
import { emptyTaskFormValues, type TaskFormValues } from "./schema"
import type { TaskFormProps } from "./types"
import TaskForm from "."

type StoryProps = {
  mode: "create" | "edit"
  initialValues?: TaskFormValues
  estimateSource?: "manual" | "action_items"
  projectOptions: TaskFormProps["options"]["projects"]
  priorityOptions: TaskFormProps["options"]["priorities"]
  hasConflict?: boolean
}

function TaskFormStory(props: StoryProps) {
  const form = useForm<TaskFormValues>({ defaultValues: props.initialValues ?? emptyTaskFormValues })
  const viewProps: Omit<TaskFormProps, "form"> = {
    heading: props.mode === "create" ? "tasks.form.createTitle" : "tasks.form.editTitle",
    submitLabel: props.mode === "create" ? "tasks.form.submit" : "tasks.form.save",
    options: { projects: props.projectOptions, priorities: props.priorityOptions },
    estimateSource: props.estimateSource,
    showTaskPriorityInheritance: props.mode === "create",
    showActionItemPriorityInheritance: true,
    showEstimateWillRecalculate: props.mode === "edit" && props.estimateSource === "action_items",
    actionItemErrors: {},
    serverError: false,
    onClearActionItemError: () => {},
    onSubmit: form.handleSubmit(() => {}),
    onCancel: () => {},
    discardConfirmation: { open: false, onConfirm: () => {}, onCancel: () => {} },
    ...(props.hasConflict ? { conflict: {
      onRequestReload: () => {},
      reloading: false,
      confirmation: { open: false, onConfirm: () => {}, onCancel: () => {} },
    } } : {}),
  }
  const [router] = useState(() => createMemoryRouter([{ path: "*", element: (
    <App><TaskForm form={form} {...viewProps} /></App>
  ) }], { initialEntries: ["/tasks/new"] }))
  return <RouterProvider router={router} />
}

const meta = {
  title: "Tasks/TaskForm",
  component: TaskFormStory,
  parameters: { layout: "fullscreen" },
  args: {
    mode: "create",
    projectOptions: [
      { value: "project-1", label: "Webサイト改善" },
      { value: "project-2", label: "プロダクトリサーチ" },
    ],
    priorityOptions: [
      { value: "urgent", label: "緊急" }, { value: "high", label: "高" }, { value: "medium", label: "中" },
      { value: "low", label: "低" }, { value: "someday", label: "いつか" },
    ],
  },
} satisfies Meta<typeof TaskFormStory>

export default meta
type Story = StoryObj<typeof meta>

export const Empty: Story = {}

export const Edit: Story = { args: { mode: "edit", initialValues: { title: "企画書を作成", projectId: "project-1", dueDate: "2026-10-18", description: "概要をまとめる", priority: "high", manualEstimate: "", actionItems: [{ seriesId: "action-1", occurrenceDate: "2026-10-10", isRecurring: false, title: "資料を集める", estimatedMinutes: "0:30", priority: "high" }] }, estimateSource: "action_items" } }

export const RecurringAndCompleted: Story = { args: { mode: "edit", initialValues: { title: "週次レポート", projectId: "project-2", dueDate: "2026-10-16", description: "今週の進捗を整理する", priority: "high", manualEstimate: "", actionItems: [
  { seriesId: "action-recurring", occurrenceDate: "2026-10-10", isRecurring: true, title: "数値を集計", estimatedMinutes: "0:30", priority: "high", completed: true },
  { seriesId: "action-once", occurrenceDate: "2026-10-10", isRecurring: false, title: "レビュー依頼を送る", estimatedMinutes: "0:15", priority: "medium", completed: false },
] }, estimateSource: "action_items" } }

export const Conflict: Story = { args: { mode: "edit", hasConflict: true } }
