import { useCallback, useLayoutEffect, useState, type ReactNode } from "react"
import { LanguageProviderContext } from "./languageContext"
import type { Language } from "./messages/types"

const LANGUAGE_STORAGE_KEY = "language"

function getInitialLanguage(): Language {
  const linked = typeof window !== "undefined" ? new URLSearchParams(window.location.search).get("lang") : null
  if (linked === "ja" || linked === "en") {
    try {
      localStorage.setItem(LANGUAGE_STORAGE_KEY, linked)
    } catch {
      // The link language still applies until the page is closed.
    }
    return linked
  }
  try {
    const saved = localStorage.getItem(LANGUAGE_STORAGE_KEY)
    if (saved === "ja" || saved === "en") return saved
  } catch {
    // Continue with the default.
  }
  return "ja"
}

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(getInitialLanguage)

  const setLanguage = useCallback((nextLanguage: Language) => {
    setLanguageState(nextLanguage)
    try {
      localStorage.setItem(LANGUAGE_STORAGE_KEY, nextLanguage)
    } catch {
      // The selected language still applies until the page is closed.
    }
  }, [])

  useLayoutEffect(() => {
    document.documentElement.lang = language
  }, [language])

  return <LanguageProviderContext.Provider value={{ language, setLanguage }}>{children}</LanguageProviderContext.Provider>
}
