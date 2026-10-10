const PERSONAL_ACCESS_TOKEN_KEY = "personal_access_token"

export function getPersonalAccessToken(): string | null {
  try {
    return localStorage.getItem(PERSONAL_ACCESS_TOKEN_KEY)
  } catch {
    return null
  }
}

export function savePersonalAccessToken(personalAccessToken: string): void {
  localStorage.setItem(PERSONAL_ACCESS_TOKEN_KEY, personalAccessToken)
}

export function clearPersonalAccessToken(): void {
  try {
    localStorage.removeItem(PERSONAL_ACCESS_TOKEN_KEY)
  } catch {
    // An unavailable localStorage should not prevent the session redirect.
  }
}
