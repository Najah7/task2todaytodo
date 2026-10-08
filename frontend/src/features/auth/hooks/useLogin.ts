import { useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { usePostLogin, type RestLoginRequest } from "~/api/generated/auth"
import { savePersonalAccessToken } from "~/features/auth/lib/localStorage"
import { SessionError } from "~/features/auth/errors"

export function useLogin() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const mutation = usePostLogin({ mutation: { retry: false, gcTime: 0 } })

  async function logIn(data: RestLoginRequest) {
    const response = await mutation.mutateAsync({ data })
    if (typeof response?.personal_access_token !== "string" || !response.personal_access_token.trim()) {
      throw new SessionError("auth.error.tokenMissing")
    }
    try {
      savePersonalAccessToken(response.personal_access_token)
    } catch {
      throw new SessionError("auth.error.tokenStorage")
    }
    await queryClient.cancelQueries()
    queryClient.clear()
    navigate("/today", { replace: true })
  }

  return { logIn, isPending: mutation.isPending }
}
