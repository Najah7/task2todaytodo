import type { messages } from "./messages"

export type Language = keyof typeof messages
export type MessageKey = keyof (typeof messages)["ja"]
