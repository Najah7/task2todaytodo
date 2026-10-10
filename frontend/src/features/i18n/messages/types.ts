import type { messages } from "./index"

export type Language = keyof typeof messages
export type MessageKey = keyof (typeof messages)["ja"]
