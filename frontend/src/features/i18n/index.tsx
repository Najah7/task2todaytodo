import { useCallback, useLayoutEffect, useState, type ReactNode } from "react"
import { LanguageProviderContext } from "./languageContext"
import type { Language } from "./types"

const LANGUAGE_STORAGE_KEY = "language"

function getInitialLanguage(): Language {
  try {
    const saved = localStorage.getItem(LANGUAGE_STORAGE_KEY)
    if (saved === "ja" || saved === "en") return saved
  } catch {
    // Use the browser language when storage is unavailable.
  }
  return typeof navigator !== "undefined" && navigator.language.toLowerCase().startsWith("ja") ? "ja" : "en"
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
