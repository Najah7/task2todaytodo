import { useState } from "react"
import type { Meta, StoryObj } from "@storybook/react-vite"
import ConfirmationDialog from "."
import controls from "~/styles/controls.module.css"

const meta = {
  title: "Projects/ConfirmationDialog",
  component: ConfirmationDialog,
} satisfies Meta<typeof ConfirmationDialog>

export default meta
type Story = StoryObj<typeof meta>

const storyArgs = {
  open: false,
  title: "Confirm action",
  description: "Review this action before continuing.",
  confirmLabel: "Continue",
  cancelLabel: "Cancel",
  onConfirm: () => {},
  onCancel: () => {},
}

function DialogExample({ destructive = false }: { destructive?: boolean }) {
  const [open, setOpen] = useState(false)

  return (
    <>
      <button className={`${controls.button} ${controls.outlineButton}`} onClick={() => setOpen(true)} type="button">
        Open confirmation
      </button>
      <ConfirmationDialog
        open={open}
        title={destructive ? "Move project to trash?" : "Discard changes?"}
        description="Unsaved changes will be lost. You can keep editing or continue."
        confirmLabel={destructive ? "Move to trash" : "Discard and leave"}
        cancelLabel="Keep editing"
        destructive={destructive}
        onConfirm={() => setOpen(false)}
        onCancel={() => setOpen(false)}
      />
    </>
  )
}

export const DiscardDraft: Story = {
  args: storyArgs,
  render: () => <DialogExample />,
}

export const MoveToTrash: Story = {
  args: storyArgs,
  render: () => <DialogExample destructive />,
}
