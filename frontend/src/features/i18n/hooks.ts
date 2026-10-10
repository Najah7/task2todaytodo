import { useCallback, useContext } from "react"
import { LanguageProviderContext } from "./languageContext"
import { messages } from "./messages"
import type { MessageKey } from "./messages/types"

export type { Language, MessageKey } from "./messages/types"

type MessageParams = Record<string, string | number>

export function useLanguage() {
  const value = useContext(LanguageProviderContext)
  if (!value) throw new Error("LanguageProvider is missing")
  return value
}

export function useI18n() {
  const { language } = useLanguage()
  return useCallback((key: MessageKey, params?: MessageParams) => {
    const template: string = messages[language][key]
    return template.replace(/\{(\w+)\}/g, (match, name: string) => String(params?.[name] ?? match))
  }, [language])
}
