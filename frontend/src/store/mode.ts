import { atom } from "jotai"

export type Mode = "light" | "dark"

export const modeAtom = atom<Mode>("light")
