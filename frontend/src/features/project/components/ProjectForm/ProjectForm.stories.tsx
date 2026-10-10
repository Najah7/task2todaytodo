import { useState } from "react"
import type { ComponentProps } from "react"
import { createMemoryRouter, RouterProvider } from "react-router"
import type { Meta, StoryObj } from "@storybook/react-vite"
import { expect, userEvent, within } from "storybook/test"
import App from "~/App"
import ProjectForm from "."
import type { ProjectFormOption } from "~/features/project/types"
import { emptyProjectFormValues } from "./schema"

const types: ProjectFormOption[] = [
  { value: "other", label: "Other" },
  { value: "work", label: "Work" },
]
const priorities: ProjectFormOption[] = [
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
]

const meta = {
  title: "Projects/ProjectForm",
  component: ProjectForm,
  parameters: { layout: "fullscreen" },
  args: {
    mode: "create",
    heading: "projects.form.createTitle",
    initialValues: emptyProjectFormValues,
    types,
    priorities,
    onSubmit: async () => ({ saved: true, revision: 2 }),
  },
} satisfies Meta<typeof ProjectForm>

export default meta
type Story = StoryObj<typeof meta>

function ProjectFormStory(props: ComponentProps<typeof ProjectForm>) {
  const [router] = useState(() => createMemoryRouter([
    { path: "*", element: <App><ProjectForm {...props} /></App> },
  ], { initialEntries: ["/projects/new"] }))
  return <RouterProvider router={router} />
}

export const Create: Story = {
  render: (args) => <ProjectFormStory {...args} />,
}

export const DateValidation: Story = {
  render: (args) => <ProjectFormStory {...args} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByLabelText(/プロジェクト名|Project name/), "Quarterly report")
    await userEvent.type(canvas.getByLabelText(/開始日|Start date/), "20261020")
    await userEvent.type(canvas.getByLabelText(/期限|Deadline/), "20261010")
    await userEvent.click(canvas.getByRole("button", { name: /作成する|Create/ }))
    await expect(await canvas.findByText(/期限は開始日以降|Deadline must be on or after/)).toBeVisible()
  },
}

export const ServerFieldValidation: Story = {
  render: () => (
    <ProjectFormStory
      mode="create"
      heading="projects.form.createTitle"
      initialValues={emptyProjectFormValues}
      types={types}
      priorities={priorities}
      onSubmit={async () => ({ saved: false, fieldErrors: { startDate: "projects.form.invalidDate" } })}
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByLabelText(/プロジェクト名|Project name/), "Quarterly report")
    await userEvent.click(canvas.getByRole("button", { name: /作成する|Create/ }))
    await expect(await canvas.findByText(/有効な日付を入力|Enter a valid date/)).toBeVisible()
  },
}

export const DirtyNavigation: Story = {
  render: (args) => <ProjectFormStory {...args} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.type(canvas.getByLabelText(/プロジェクト名|Project name/), "A draft")
    await userEvent.click(canvas.getByRole("button", { name: /キャンセル|Cancel/ }))
    await expect(await canvas.findByRole("alertdialog", { name: /変更を破棄|Discard changes/ })).toBeVisible()
    await userEvent.click(canvas.getByRole("button", { name: /編集を続ける|Keep editing/ }))
    await expect(canvas.getByLabelText(/プロジェクト名|Project name/)).toHaveValue("A draft")
  },
}
