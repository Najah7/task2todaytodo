import { afterEach, describe, expect, it, vi } from "vitest"
import { addTaskToToday, makeTaskUrgent } from "./addActionItemToToday"

vi.mock("~/features/shared/notification", () => ({ notify: { success: vi.fn(), error: vi.fn(), action: vi.fn() } }))

afterEach(() => vi.unstubAllGlobals())

describe("addTaskToToday", () => {
  it("adds direct Task work using latest quoted Today revision", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ revision: 12 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ revision: 13, items: [] }), { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)

    await addTaskToToday("task-7")

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "/api/today-todo",
      "/api/today-todo/tasks/task-7",
    ])
    const [, request] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(new Headers(request.headers).get("If-Match")).toBe('"12"')
  })
})

describe("makeTaskUrgent", () => {
  it("patches Task priority without treating it as an ActionItem", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ task: { id: "task-1", revision: 7 } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: "task-1" }), { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)

    await makeTaskUrgent((key) => key, "task-1")

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(["/api/tasks/task-1", "/api/tasks/task-1"])
    const [, request] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(JSON.parse(String(request.body))).toEqual({ priority: "urgent" })
    expect(new Headers(request.headers).get("If-Match")).toBe('"7"')
  })
})
