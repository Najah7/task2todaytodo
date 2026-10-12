import { fireEvent, render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import { LanguageProvider } from "~/features/i18n"
import InvitationAcceptForm from "./index"

describe("InvitationAcceptForm", () => {
  it("requires explicit acceptance for a verified invited account", () => {
    const accept = vi.fn()
    render(
      <LanguageProvider>
        <InvitationAcceptForm
          email="member@example.com"
          mode="verified"
          signedIn
          tokenHash="#token=secret"
          onAcceptWithPassword={vi.fn()}
          onLogin={vi.fn()}
          onAcceptSignedIn={accept}
          onSwitchAccount={vi.fn()}
        />
      </LanguageProvider>,
    )

    fireEvent.click(screen.getByRole("button", { name: "招待を受け入れる" }))
    expect(accept).toHaveBeenCalledOnce()
    expect(screen.getByText("招待先: member@example.com")).toBeTruthy()
  })
})
