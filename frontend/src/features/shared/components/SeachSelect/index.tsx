import { forwardRef, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import { createPortal } from "react-dom"
import { useI18n } from "~/features/i18n/hooks"
import styles from "./index.module.css"

export type SeachSelectOption = { value: string; label: string }

type Props = {
  id?: string
  value: string
  options: SeachSelectOption[]
  onChange: (value: string) => void
  labelledBy?: string
  ariaLabel?: string
  describedBy?: string
  className?: string
  disabled?: boolean
  invalid?: boolean
  onBlur?: () => void
}

type PopupPosition = { top: number; left: number; width: number }

const SeachSelect = forwardRef<HTMLButtonElement, Props>(function SeachSelect({
  id,
  value,
  options,
  onChange,
  labelledBy,
  ariaLabel,
  describedBy,
  className,
  disabled = false,
  invalid = false,
  onBlur,
}, forwardedRef) {
  const i18n = useI18n()
  const generatedId = useId()
  const controlId = id ?? generatedId
  const listboxId = `${generatedId}-listbox`
  const selectedValueId = `${generatedId}-selected-value`
  const triggerRef = useRef<HTMLButtonElement | null>(null)
  const searchRef = useRef<HTMLInputElement | null>(null)
  const popupRef = useRef<HTMLDivElement | null>(null)
  const optionRefs = useRef<Array<HTMLLIElement | null>>([])
  const composing = useRef(false)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [activeIndex, setActiveIndex] = useState(-1)
  const [position, setPosition] = useState<PopupPosition | null>(null)

  const selectedLabel = options.find((option) => option.value === value)?.label ?? value
  const filteredOptions = useMemo(() => {
    const term = query.trim().toLocaleLowerCase()
    if (!term) return options
    return options.filter((option) => option.label.toLocaleLowerCase().includes(term))
  }, [options, query])
  const activeOption = filteredOptions[activeIndex]

  function setTriggerRef(element: HTMLButtonElement | null) {
    triggerRef.current = element
    if (typeof forwardedRef === "function") forwardedRef(element)
    else if (forwardedRef) (forwardedRef as { current: HTMLButtonElement | null }).current = element
  }

  function openPopup() {
    if (disabled) return
    setQuery("")
    const selectedIndex = options.findIndex((option) => option.value === value)
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : 0)
    setPosition(null)
    setOpen(true)
  }

  function closePopup(restoreFocus: boolean) {
    setOpen(false)
    setQuery("")
    setPosition(null)
    if (restoreFocus) queueMicrotask(() => triggerRef.current?.focus())
  }

  function tokenLengthInPixels(token: string, fallback: number) {
    const rootStyle = getComputedStyle(document.documentElement)
    const value = rootStyle.getPropertyValue(token).trim()
    const amount = Number.parseFloat(value)
    if (!Number.isFinite(amount)) return fallback
    return value.endsWith("rem") ? amount * Number.parseFloat(rootStyle.fontSize) : amount
  }

  function choose(option: SeachSelectOption) {
    if (option.value !== value) onChange(option.value)
    closePopup(true)
  }

  function handleSearchKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Tab") {
      closePopup(false)
      // Let the browser's regular tab order continue from the trigger instead of
      // reconstructing focus order while the search input is portaled.
      triggerRef.current?.focus()
      return
    }

    const isComposing = composing.current || event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229
    if (isComposing && ["Escape", "ArrowDown", "ArrowUp", "Enter"].includes(event.key)) return

    if (event.key === "Escape") {
      event.preventDefault()
      closePopup(true)
      return
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault()
      if (!filteredOptions.length) return
      setActiveIndex((current) => {
        const direction = event.key === "ArrowDown" ? 1 : -1
        return (current + filteredOptions.length + direction) % filteredOptions.length
      })
      return
    }

    if (event.key === "Enter") {
      event.preventDefault()
      const option = activeOption ?? filteredOptions[0]
      if (option) choose(option)
    }
  }

  useEffect(() => {
    if (!open) return
    searchRef.current?.focus()
    const dismissOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      if (triggerRef.current?.contains(target) || popupRef.current?.contains(target)) return
      closePopup(false)
      onBlur?.()
    }
    document.addEventListener("pointerdown", dismissOnOutsidePointer, true)
    return () => document.removeEventListener("pointerdown", dismissOnOutsidePointer, true)
  }, [open, onBlur])

  useEffect(() => {
    if (disabled && open) closePopup(false)
  }, [disabled, open])

  useLayoutEffect(() => {
    if (!open) return
    const updatePosition = () => {
      const trigger = triggerRef.current
      const popup = popupRef.current
      if (!trigger || !popup) return
      const rect = trigger.getBoundingClientRect()
      const popupHeight = popup.getBoundingClientRect().height
      const margin = tokenLengthInPixels("--space-sm", 8)
      const gap = tokenLengthInPixels("--space-xs", 4)
      const minWidth = tokenLengthInPixels("--size-search-width", 224)
      const width = Math.min(Math.max(rect.width, minWidth), Math.max(0, window.innerWidth - margin * 2))
      const left = Math.max(margin, Math.min(rect.left, window.innerWidth - width - margin))
      const below = window.innerHeight - rect.bottom - margin
      const above = rect.top - margin
      const placeBelow = below >= popupHeight || below >= above
      const preferredTop = placeBelow ? rect.bottom + gap : rect.top - popupHeight - gap
      const top = Math.max(margin, Math.min(preferredTop, window.innerHeight - popupHeight - margin))
      setPosition({ top, left, width })
    }

    updatePosition()
    window.addEventListener("resize", updatePosition)
    window.addEventListener("scroll", updatePosition, true)
    return () => {
      window.removeEventListener("resize", updatePosition)
      window.removeEventListener("scroll", updatePosition, true)
    }
  }, [open, filteredOptions.length])

  useEffect(() => {
    if (!open || activeIndex < 0) return
    optionRefs.current[activeIndex]?.scrollIntoView?.({ block: "nearest" })
  }, [activeIndex, open])

  const searchPopup = open && createPortal(
    <div
      ref={popupRef}
      className={styles.popup}
      style={{
        top: position?.top ?? 0,
        left: position?.left ?? 0,
        width: position?.width ?? 0,
        visibility: position ? "visible" : "hidden",
      }}
      onClick={(event) => event.stopPropagation()}
    >
      <div className={styles.searchHeader}>
        <svg className={styles.searchIcon} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle cx="10.8" cy="10.8" r="6.3" />
          <path d="m15.5 15.5 4.2 4.2" />
        </svg>
        <input
          ref={searchRef}
          className={styles.searchInput}
          type="text"
          role="combobox"
          disabled={disabled}
          aria-labelledby={labelledBy}
          aria-label={labelledBy ? undefined : ariaLabel}
          aria-description={i18n("shared.select.search")}
          aria-autocomplete="list"
          aria-expanded="true"
          aria-controls={listboxId}
          aria-activedescendant={activeOption ? `${generatedId}-option-${activeIndex}` : undefined}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          autoComplete="off"
          placeholder={i18n("shared.select.search")}
          value={query}
          onChange={(event) => {
            setQuery(event.currentTarget.value)
            setActiveIndex(0)
          }}
          onKeyDown={handleSearchKeyDown}
          onCompositionStart={() => { composing.current = true }}
          onCompositionEnd={() => { composing.current = false }}
          onBlur={onBlur}
        />
      </div>
      <ul
        id={listboxId}
        className={styles.options}
        role="listbox"
        aria-labelledby={labelledBy}
        aria-label={labelledBy ? undefined : ariaLabel}
      >
        {filteredOptions.length ? filteredOptions.map((option, index) => (
          <li
            key={option.value}
            ref={(element) => { optionRefs.current[index] = element }}
            id={`${generatedId}-option-${index}`}
            className={styles.option}
            role="option"
            aria-selected={option.value === value}
            data-active={index === activeIndex || undefined}
            onMouseEnter={() => setActiveIndex(index)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => choose(option)}
          >
            <span>{highlightMatch(option.label, query)}</span>
            {option.value === value && (
              <svg className={styles.checkIcon} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                <path d="m5 12.5 4.2 4.2L19 7" />
              </svg>
            )}
          </li>
        )) : (
          <li className={styles.empty} role="presentation">
            <span role="status">{i18n("shared.select.empty")}</span>
          </li>
        )}
      </ul>
    </div>,
    document.body,
  )

  return (
    <>
      <button
        ref={setTriggerRef}
        id={controlId}
        className={`${styles.trigger} ${className ?? ""}`}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listboxId}
        aria-labelledby={labelledBy ? `${labelledBy} ${selectedValueId}` : undefined}
        aria-label={labelledBy ? undefined : ariaLabel ? `${ariaLabel} ${selectedLabel}` : undefined}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        disabled={disabled}
        onClick={(event) => {
          event.stopPropagation()
          if (open) closePopup(true)
          else openPopup()
        }}
        onKeyDown={(event) => {
          event.stopPropagation()
          if (!event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229 && ["ArrowDown", "ArrowUp", "Enter", " "].includes(event.key)) {
            event.preventDefault()
            openPopup()
          }
        }}
        onBlur={onBlur}
      >
        <span id={selectedValueId} className={styles.value}>{selectedLabel}</span>
        <svg className={styles.chevron} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      {searchPopup}
    </>
  )
})

export default SeachSelect

function highlightMatch(label: string, query: string) {
  const term = query.trim()
  if (!term) return label
  const start = label.toLocaleLowerCase().indexOf(term.toLocaleLowerCase())
  if (start < 0) return label
  const end = start + term.length
  return <>{label.slice(0, start)}<mark>{label.slice(start, end)}</mark>{label.slice(end)}</>
}
