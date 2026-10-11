import { useContext } from "react"
import { act, renderHook } from "@testing-library/react"
import { MemoryRouter, useLocation, useNavigate } from "react-router"
import { describe, expect, it } from "vitest"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import { InboxNavigationProvider } from "~/features/inbox/providers/InboxNavigationProvider"

describe("InboxNavigationProvider", () => {
  it("preserves rapid type, project, and title changes in the URL", () => {
    const { result } = renderHook(() => ({
      navigation: useContext(InboxNavigationContext)!,
      search: useLocation().search,
    }), {
      wrapper: ({ children }) => <MemoryRouter initialEntries={["/inbox?keep=1&page_token=old"]}>
        <InboxNavigationProvider>{children}</InboxNavigationProvider>
      </MemoryRouter>,
    })

    act(() => {
      result.current.navigation.selectType("schedule")
      result.current.navigation.selectProject("project-1")
      result.current.navigation.changeTitle("MULTI-DAY")
    })

    expect(result.current.search).toBe("?keep=1&type=schedule&project_id=project-1&title=MULTI-DAY")
    expect(result.current.navigation.state).toMatchObject({
      type: "schedule",
      projectId: "project-1",
      title: "MULTI-DAY",
      pageToken: undefined,
    })
  })

  it("preserves rapid changes while clearing or replacing the cursor", () => {
    const { result } = renderHook(() => ({
      navigation: useContext(InboxNavigationContext)!,
      search: useLocation().search,
    }), {
      wrapper: ({ children }) => <MemoryRouter initialEntries={["/inbox?type=task&page_token=old"]}>
        <InboxNavigationProvider>{children}</InboxNavigationProvider>
      </MemoryRouter>,
    })

    act(() => {
      result.current.navigation.selectProject("no-project")
      result.current.navigation.next("next-token")
    })

    expect(result.current.search).toBe("?type=task&project_id=no-project&page_token=next-token")
  })

  it("uses browser-restored filters as the base for the next change", () => {
    const { result } = renderHook(() => ({
      navigation: useContext(InboxNavigationContext)!,
      search: useLocation().search,
      navigate: useNavigate(),
    }), {
      wrapper: ({ children }) => <MemoryRouter initialEntries={["/inbox?type=task&project_id=project-1&title=Old"]}>
        <InboxNavigationProvider>{children}</InboxNavigationProvider>
      </MemoryRouter>,
    })

    act(() => result.current.navigation.selectType("schedule"))
    expect(result.current.search).toBe("?type=schedule&project_id=project-1&title=Old")

    act(() => result.current.navigate(-1))
    expect(result.current.search).toBe("?type=task&project_id=project-1&title=Old")

    act(() => result.current.navigation.selectProject("no-project"))
    expect(result.current.search).toBe("?type=task&project_id=no-project&title=Old")
  })
})
