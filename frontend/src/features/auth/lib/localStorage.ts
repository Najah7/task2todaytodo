export const ACCESS_TOKEN_KEY = "access_token"

export function getPAT(): string | null {
  try {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  } catch {
    return null
  }
}

export function savePAT(token: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, token)
}
