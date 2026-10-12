import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { LanguageProvider } from "~/features/i18n"
import MemberRoleTable from "./index"

describe("MemberRoleTable", () => {
  it("renders backend capability values for each role", () => {
    render(
      <LanguageProvider>
        <MemberRoleTable
          roles={[{ id: "owner" }, { id: "admin" }, { id: "editor" }, { id: "viewer" }]}
          capabilities={[{
            id: "project_member.manage_admin",
            owner: true,
            admin: false,
            editor: false,
            viewer: false,
          }]}
        />
      </LanguageProvider>,
    )

    const capabilityRow = screen.getByRole("row", { name: /管理者の権限を管理する/ })
    expect(capabilityRow.textContent).toContain("✓")
    expect(capabilityRow.textContent?.match(/—/g)).toHaveLength(3)
    expect(screen.getAllByRole("columnheader")).toHaveLength(5)
  })
})
