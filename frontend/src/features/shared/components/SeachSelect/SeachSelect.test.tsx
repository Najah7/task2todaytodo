import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { useState } from "react"
import { afterEach, expect, test, vi } from "vitest"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import SeachSelect, { type SeachSelectOption } from "."

const options: SeachSelectOption[] = [
  { value: "work", label: "Work" },
  { value: "workshop", label: "Workshop" },
  { value: "personal", label: "Personal" },
]

afterEach(cleanup)

function renderSelect({ disabled = false, invalid = false }: { disabled?: boolean; invalid?: boolean } = {}) {
  function Demo() {
    const [value, setValue] = useState("personal")
    return (
      <>
        <button type="button" id="previous">Previous</button>
        <label id="select-label" htmlFor="test-select">Project type</label>
        <SeachSelect
          id="test-select"
          labelledBy="select-label"
          describedBy={invalid ? "select-error" : undefined}
          value={value}
          options={options}
          onChange={setValue}
          disabled={disabled}
          invalid={invalid}
        />
        {invalid && <p id="select-error">Choose a project type.</p>}
        <input aria-label="Next field" />
      </>
    )
  }

  return render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
      <Demo />
    </LanguageProviderContext.Provider>,
  )
}

test("filters partial matches, highlights them, and selects the active option with arrows and Enter", async () => {
  renderSelect()
  const trigger = screen.getByRole("button", { name: "Project type Personal" })
  fireEvent.click(trigger)
  const search = await screen.findByRole("combobox", { name: "Project type" })
  fireEvent.change(search, { target: { value: "wo" } })

  const workshop = screen.getByRole("option", { name: "Workshop" })
  expect(workshop.querySelector("mark")?.textContent).toBe("Wo")
  fireEvent.keyDown(search, { key: "ArrowDown" })
  fireEvent.keyDown(search, { key: "Enter" })

  await waitFor(() => expect(screen.getByRole("button", { name: "Project type Workshop" })).toBeTruthy())
  expect(screen.queryByRole("listbox")).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Project type Workshop" }))
})

test("keeps an IME Enter from selecting until composition ends and reports empty results", async () => {
  renderSelect()
  fireEvent.click(screen.getByRole("button", { name: "Project type Personal" }))
  const search = await screen.findByRole("combobox", { name: "Project type" })
  fireEvent.change(search, { target: { value: "missing" } })
  expect(screen.getByRole("status").textContent).toBe("No matching options.")

  fireEvent.change(search, { target: { value: "work" } })
  fireEvent.compositionStart(search)
  const activeOptionId = search.getAttribute("aria-activedescendant")
  fireEvent.keyDown(search, { key: "ArrowDown", isComposing: true })
  fireEvent.keyDown(search, { key: "Escape", isComposing: true })
  fireEvent.keyDown(search, { key: "Enter", isComposing: true })
  expect(screen.getByRole("listbox")).toBeTruthy()
  expect(search.getAttribute("aria-activedescendant")).toBe(activeOptionId)
  expect(screen.getByRole("button", { name: "Project type Personal" })).toBeTruthy()

  fireEvent.compositionEnd(search)
  fireEvent.keyDown(search, { key: "Enter" })
  await waitFor(() => expect(screen.getByRole("button", { name: "Project type Work" })).toBeTruthy())
})

test("Escape restores trigger focus and Tab is left to the browser's page order", async () => {
  renderSelect()
  const trigger = screen.getByRole("button", { name: "Project type Personal" })
  fireEvent.click(trigger)
  let search = await screen.findByRole("combobox", { name: "Project type" })
  fireEvent.keyDown(search, { key: "Escape" })
  await waitFor(() => expect(document.activeElement).toBe(trigger))

  fireEvent.click(trigger)
  search = await screen.findByRole("combobox", { name: "Project type" })
  const tabWasHandledByBrowser = fireEvent.keyDown(search, { key: "Tab" })
  expect(tabWasHandledByBrowser).toBe(true)
  expect(document.activeElement).toBe(trigger)

  trigger.focus()
  fireEvent.click(trigger)
  search = await screen.findByRole("combobox", { name: "Project type" })
  const shiftTabWasHandledByBrowser = fireEvent.keyDown(search, { key: "Tab", shiftKey: true })
  expect(shiftTabWasHandledByBrowser).toBe(true)
  expect(document.activeElement).toBe(trigger)
})

test("outside pointer dismisses the popup", async () => {
  renderSelect()
  const trigger = screen.getByRole("button", { name: "Project type Personal" })
  fireEvent.click(trigger)
  await screen.findByRole("combobox", { name: "Project type" })
  const previous = screen.getByRole("button", { name: "Previous" })
  fireEvent.pointerDown(previous)
  expect(screen.queryByRole("listbox")).toBeNull()
})

test("reselecting the current option closes without emitting a change", async () => {
  const onChange = vi.fn()
  render(
    <LanguageProviderContext.Provider value={{ language: "en", setLanguage: () => undefined }}>
      <SeachSelect value="personal" options={options} onChange={onChange} ariaLabel="Project type" />
    </LanguageProviderContext.Provider>,
  )
  fireEvent.click(screen.getByRole("button", { name: "Project type Personal" }))
  await screen.findByRole("combobox", { name: "Project type" })
  fireEvent.click(screen.getByRole("option", { name: "Personal" }))
  expect(onChange).not.toHaveBeenCalled()
  expect(screen.queryByRole("listbox")).toBeNull()
})

test("disabled and invalid states are exposed to assistive technology", async () => {
  renderSelect({ disabled: true, invalid: true })
  const trigger = screen.getByRole("button", { name: "Project type Personal" })
  expect((trigger as HTMLButtonElement).disabled).toBe(true)
  expect(trigger.getAttribute("aria-invalid")).toBe("true")
  expect(trigger.getAttribute("aria-describedby")).toBe("select-error")
  fireEvent.click(trigger)
  expect(screen.queryByRole("combobox")).toBeNull()

  cleanup()
  renderSelect({ invalid: true })
  fireEvent.click(screen.getByRole("button", { name: "Project type Personal" }))
  const search = await screen.findByRole("combobox", { name: "Project type" })
  expect(search.getAttribute("aria-invalid")).toBe("true")
  expect(search.getAttribute("aria-describedby")).toBe("select-error")
})
