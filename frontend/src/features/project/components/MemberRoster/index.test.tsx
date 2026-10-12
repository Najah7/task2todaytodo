import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import { LanguageProvider } from "~/features/i18n"
import MemberRoster from "./index"

describe("MemberRoster", () => {
  it("keeps owner immutable and exposes actions for pending and removable members", () => {
    render(
      <LanguageProvider>
        <MemberRoster
          items={[
            { key: "owner", name: "Owner", email: "owner@example.com", roleId: "owner", status: "owner", canEditRole: false, canRemove: false },
            { key: "pending", name: "invitee@example.com", email: "invitee@example.com", roleId: "viewer", status: "pending", canEditRole: false, canRemove: true },
            { key: "admin-invite", name: "admin@example.com", email: "admin@example.com", roleId: "admin", status: "expired", canEditRole: false, canRemove: false },
            { key: "editor", name: "Editor", email: "editor@example.com", roleId: "editor", status: "accepted", canEditRole: false, canRemove: true },
          ]}
          roles={[{ id: "admin" }, { id: "editor" }, { id: "viewer" }]}
          canManageInvitations
          onRoleChange={vi.fn()}
          onRemove={vi.fn()}
          onResend={vi.fn()}
          onRevoke={vi.fn()}
        />
      </LanguageProvider>,
    )

    expect(screen.getAllByText("オーナー")).toHaveLength(2)
    expect(screen.getAllByRole("button", { name: "再送" })).toHaveLength(1)
    expect(screen.getByRole("button", { name: "招待を取り消す" })).toBeTruthy()
    expect(screen.getByRole("button", { name: "メンバーを削除" })).toBeTruthy()
    expect(screen.queryByRole("combobox")).toBeNull()
  })
})
