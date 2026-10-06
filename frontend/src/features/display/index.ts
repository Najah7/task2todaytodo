import { useAtom, getDefaultStore } from "jotai"
import { modeAtom, type Mode } from "~/store/mode"

const DISPLAY_STORAGE_KEY = "display_mode"

function getInitialMode(): Mode {
  try {
    const saved = localStorage.getItem(DISPLAY_STORAGE_KEY)
    if (saved === "light" || saved === "dark") return saved
  } catch {
    // Fall back to the OS preference when storage is unavailable.
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

export function initializeDisplay() {
  const mode = getInitialMode()
  document.documentElement.dataset.display = mode
  getDefaultStore().set(modeAtom, mode)
}

export function useDisplay() {
  const [mode, setMode] = useAtom(modeAtom)

  function setDisplay(nextMode: Mode) {
    document.documentElement.dataset.display = nextMode
    setMode(nextMode)
    try {
      localStorage.setItem(DISPLAY_STORAGE_KEY, nextMode)
    } catch {
      // The selection still applies to this session.
    }
  }

  return { mode, setDisplay }
}
