import { clearPersonalAccessToken, getPersonalAccessToken } from "~/features/auth/lib/localStorage"

export class ApiError<T = unknown> extends Error {
  readonly status: number
  readonly data: T

  constructor(status: number, data: T) {
    super("API request failed")
    this.name = "ApiError"
    this.status = status
    this.data = data
  }
}

export function isTodoRegenerationRequired(error: unknown): error is ApiError {
  if (!(error instanceof ApiError) || error.status !== 409 || !isRecord(error.data)) return false
  const body = error.data.error
  if (!isRecord(body) || !Array.isArray(body.details)) return false
  return body.details.some((detail) => isRecord(detail) && detail.code === "todo_regeneration_required")
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

export type ErrorType<T> = ApiError<T>

export async function apiFetch<T>(url: string, options: RequestInit): Promise<T> {
  const headers = new Headers(options.headers)
  const token = getPersonalAccessToken()
  if (token) headers.set("Authorization", `Bearer ${token}`)
  headers.set("Accept", "application/json")

  const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "")
  const response = await fetch(`${baseUrl}${url}`, { ...options, headers })
  if (response.status === 401 && token && !url.startsWith("/api/login")) {
    clearPersonalAccessToken()
    window.dispatchEvent(new Event("auth:unauthorized"))
  }
  const text = await response.text()
  let data: unknown
  try {
    data = text ? JSON.parse(text) : undefined
  } catch {
    throw new ApiError(response.status, undefined)
  }
  if (!response.ok) throw new ApiError(response.status, data)
  return data as T
}
