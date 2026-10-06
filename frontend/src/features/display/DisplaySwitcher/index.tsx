import { useDisplay } from "~/features/display"
import { useI18n } from "~/features/i18n/hooks"
import Switcher from "~/features/shared/components/Switcher"

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeLinecap="round" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M2 12h2m16 0h2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42" />
    </svg>
  )
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false">
      <path d="M20.9 13.3A9 9 0 0 1 10.7 3.1a9 9 0 1 0 10.2 10.2Z" />
    </svg>
  )
}

export default function DisplaySwitcher() {
  const { mode, setDisplay } = useDisplay()
  const i18n = useI18n()

  return (
    <Switcher
      label={i18n("common.display")}
      leftLabel={i18n("common.display.light")}
      rightLabel={i18n("common.display.dark")}
      left={<SunIcon />}
      right={<MoonIcon />}
      rightSelected={mode === "dark"}
      onChange={(rightSelected) => setDisplay(rightSelected ? "dark" : "light")}
    />
  )
}
