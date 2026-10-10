import { useCallback, useRef } from "react"
import { useBeforeUnload, useBlocker } from "react-router"

/** Protect an edited Project draft from in-app navigation and browser unload. */
export function useProjectDraftGuard(isDirty: boolean) {
  const dirtyRef = useRef(isDirty)
  dirtyRef.current = isDirty
  const shouldBlock = useCallback(() => dirtyRef.current, [])
  const blocker = useBlocker(shouldBlock)

  useBeforeUnload(
    useCallback((event: BeforeUnloadEvent) => {
      if (!dirtyRef.current) return
      event.preventDefault()
      event.returnValue = ""
    }, []),
  )

  return {
    dialogOpen: blocker.state === "blocked",
    stay: () => blocker.reset?.(),
    leave: () => blocker.proceed?.(),
    markClean: () => { dirtyRef.current = false },
  }
}
