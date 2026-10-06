import { useContext } from "react"
import { LanguageProviderContext } from "./languageContext"
import { messages, type MessageKey, type MessageParams } from "./messages"

export type { Language, MessageKey, MessageParams } from "./messages"

export function useLanguage() {
  const value = useContext(LanguageProviderContext)
  if (!value) throw new Error("LanguageProvider is missing")
  return value
}

export function useI18n() {
  const { language } = useLanguage()
  return (key: MessageKey, params?: MessageParams) => {
    const template: string = messages[language][key]
    return template.replace(/\{(\w+)\}/g, (match, name: string) => String(params?.[name] ?? match))
  }
}
