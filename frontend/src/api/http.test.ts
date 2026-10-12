import { beforeEach, describe, expect, it, vi } from "vitest"
import { ApiError, apiFetch, isTodoRegenerationRequired } from "./http"

describe("Todo regeneration confirmation errors", () => {
  it("recognizes only the explicit confirmation-required detail on HTTP 409", () => {
    expect(isTodoRegenerationRequired(new ApiError(409, { error: { details: [{ code: "todo_regeneration_required" }] } }))).toBe(true)
    expect(isTodoRegenerationRequired(new ApiError(409, { error: { details: [{ code: "revision_conflict" }] } }))).toBe(false)
    expect(isTodoRegenerationRequired(new ApiError(412, { error: { details: [{ code: "todo_regeneration_required" }] } }))).toBe(false)
    expect(isTodoRegenerationRequired(new Error("failed"))).toBe(false)
  })
})

describe("apiFetch session handling", () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it("clears an invalid saved token and signals the router to redirect", async () => {
    localStorage.setItem("personal_access_token", "expired-token")
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      status: 401,
      ok: false,
      text: async () => JSON.stringify({ error: "unauthorized" }),
    }))
    const unauthorized = vi.fn()
    window.addEventListener("auth:unauthorized", unauthorized)

    await expect(apiFetch("/api/projects", {})).rejects.toBeInstanceOf(ApiError)

    expect(localStorage.getItem("personal_access_token")).toBeNull()
    expect(unauthorized).toHaveBeenCalledOnce()
    window.removeEventListener("auth:unauthorized", unauthorized)
  })

  it("leaves login credential errors on the login page", async () => {
    localStorage.setItem("personal_access_token", "expired-token")
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      status: 401,
      ok: false,
      text: async () => JSON.stringify({ error: "invalid credentials" }),
    }))
    const unauthorized = vi.fn()
    window.addEventListener("auth:unauthorized", unauthorized)

    await expect(apiFetch("/api/login", { method: "POST" })).rejects.toBeInstanceOf(ApiError)

    expect(localStorage.getItem("personal_access_token")).toBe("expired-token")
    expect(unauthorized).not.toHaveBeenCalled()
    window.removeEventListener("auth:unauthorized", unauthorized)
  })
})
