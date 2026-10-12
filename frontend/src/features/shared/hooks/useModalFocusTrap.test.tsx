import { afterEach, expect, test } from "vitest"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { useState } from "react"
import { useModalFocusTrap } from "./useModalFocusTrap"

afterEach(cleanup)

function TestModal({ form }: { form: boolean }) {
  const [open, setOpen] = useState(false)
  const modal = useModalFocusTrap<HTMLElement>(open, () => setOpen(false))
  return <>
    <button type="button" onClick={() => setOpen(true)}>Open dialog</button>
    {open && <section ref={modal.ref} onKeyDown={modal.onKeyDown} role="dialog" aria-label="Test dialog">
      <button type="button">Close</button>
      {form && <form><label>First field<input aria-label="First field" /></label></form>}
    </section>}
  </>
}

test("focuses the first enabled form control before an earlier dialog button", () => {
  render(<TestModal form />)
  fireEvent.click(screen.getByRole("button", { name: "Open dialog" }))

  expect(document.activeElement).toBe(screen.getByLabelText("First field"))
})

test("focuses a dialog control when the dialog has no form", () => {
  render(<TestModal form={false} />)
  fireEvent.click(screen.getByRole("button", { name: "Open dialog" }))

  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Close" }))
})
