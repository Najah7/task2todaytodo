import { useCallback, useEffect, useRef, type KeyboardEvent } from "react"

const focusableSelector = "button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex='-1'])"

export function useModalFocusTrap<T extends HTMLElement>(open: boolean, onEscape: () => void) {
  const ref = useRef<T | null>(null)
  const escapeRef = useRef(onEscape)
  escapeRef.current = onEscape

  useEffect(() => {
    if (!open || !ref.current) return
    const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const initial = ref.current.querySelector<HTMLElement>("form select:not([disabled]), form input:not([disabled]), form button:not([disabled])")
      ?? ref.current.querySelector<HTMLElement>("button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex='-1'])")
    initial?.focus()
    return () => {
      if (previouslyFocused?.isConnected) previouslyFocused.focus()
    }
  }, [open])

  const onKeyDown = useCallback((event: KeyboardEvent<T>) => {
    if (event.key === "Escape") {
      event.preventDefault()
      escapeRef.current()
      return
    }
    if (event.key !== "Tab" || !ref.current) return
    const elements = [...ref.current.querySelectorAll<HTMLElement>(focusableSelector)]
    const first = elements[0]
    const last = elements.at(-1)
    if (!first || !last) return
    if (event.shiftKey && (document.activeElement === first || !ref.current.contains(document.activeElement))) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && (document.activeElement === last || !ref.current.contains(document.activeElement))) {
      event.preventDefault()
      first.focus()
    }
  }, [])

  return { ref, onKeyDown }
}
