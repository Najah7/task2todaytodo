import { useNavigate } from "react-router"
import { usePostLogin, type InternalPortRestLoginRequest } from "~/api/generated/auth"
import { savePAT } from "~/features/auth/lib/localStorage"
import { SessionError } from "~/features/auth/errors"

export function useLogin() {
  const navigate = useNavigate()
  const mutation = usePostLogin({ mutation: { retry: false, gcTime: 0 } })

  async function logIn(data: InternalPortRestLoginRequest) {
    const response = await mutation.mutateAsync({ data })
    if (typeof response?.token !== "string" || !response.token.trim()) {
      throw new SessionError("auth.error.tokenMissing")
    }
    try {
      savePAT(response.token)
    } catch {
      throw new SessionError("auth.error.tokenStorage")
    }
    navigate("/today", { replace: true })
  }

  return { logIn, isPending: mutation.isPending }
}
