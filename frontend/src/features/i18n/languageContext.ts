import { createContext } from "react"
import type { Language } from "./messages"

export type LanguageContextValue = {
  language: Language
  setLanguage: (language: Language) => void
}

export const LanguageProviderContext = createContext<LanguageContextValue | null>(null)
