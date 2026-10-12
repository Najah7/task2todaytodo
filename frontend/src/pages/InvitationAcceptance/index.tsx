import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocation, useNavigate } from "react-router"
import { getUsersMe, postInvitationsAccept, postInvitationsPreview, postLogin } from "~/api/generated/auth"
import { ApiError } from "~/api/http"
import AuthActionPanel from "~/features/auth/components/AuthActionPanel"
import { getAuthErrorMessage } from "~/features/auth/errors"
import { getPersonalAccessToken, clearPersonalAccessToken, savePersonalAccessToken } from "~/features/auth/lib/localStorage"
import InvitationAcceptForm from "~/features/project/components/InvitationAcceptForm"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import AuthPageFrame from "~/features/auth/components/AuthPageFrame"
import { useEmailActionToken } from "~/features/auth/hooks/useEmailActionToken"
import styles from "./index.module.css"

export default function InvitationAcceptancePage() {
  const i18n = useI18n()
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const token = useEmailActionToken()
  const [hasSession, setHasSession] = useState(() => Boolean(getPersonalAccessToken()))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)
  const previewQuery = useQuery({
    queryKey: ["projectInvitationPreview", token],
    queryFn: () => postInvitationsPreview({ token: token! }),
    enabled: Boolean(token),
    retry: false,
  })
  const userQuery = useQuery({ queryKey: ["currentUser"], queryFn: getUsersMe, enabled: hasSession, retry: false })
  const invitation = previewQuery.data
  const email = invitation?.email ?? ""
  const currentEmail = userQuery.data?.email?.trim().toLocaleLowerCase()
  const accountMismatch = Boolean(hasSession && currentEmail && email && currentEmail !== email.trim().toLocaleLowerCase())
  const signedIn = Boolean(hasSession && currentEmail && !accountMismatch)
  const mode = invitation?.recipient_status ?? "verified"
  const isPending = invitation?.status === "pending"
  const invalid = !token || previewQuery.isError || (invitation && !isPending)

  async function clearSession() {
    clearPersonalAccessToken()
    setHasSession(false)
    await queryClient.cancelQueries()
    queryClient.clear()
  }

  async function accept(password?: string) {
    if (!token || !invitation || !isPending || busy) return
    setBusy(true)
    setError(null)
    try {
      const response = await postInvitationsAccept({ token, password })
      const accessToken = response?.personal_access_token
      if (typeof accessToken === "string" && accessToken.trim()) savePersonalAccessToken(accessToken)
      await queryClient.cancelQueries()
      queryClient.clear()
      navigate(`/members?projectId=${encodeURIComponent(invitation.project_id ?? "")}`, { replace: true })
    } catch {
      setError("members.invitation.acceptError")
    } finally {
      setBusy(false)
    }
  }

  async function loginAndAccept(password: string) {
    if (!invitation || busy) return
    setBusy(true)
    setError(null)
    try {
      const login = await postLogin({ email, password })
      if (!login.personal_access_token?.trim()) throw new Error("auth.error.tokenMissing")
      savePersonalAccessToken(login.personal_access_token)
      setHasSession(true)
      const response = await postInvitationsAccept({ token: token! })
      const accessToken = response?.personal_access_token
      if (typeof accessToken === "string" && accessToken.trim()) savePersonalAccessToken(accessToken)
      await queryClient.cancelQueries()
      queryClient.clear()
      navigate(`/members?projectId=${encodeURIComponent(invitation.project_id ?? "")}`, { replace: true })
    } catch (cause) {
      setError(cause instanceof ApiError ? getAuthErrorMessage(cause) : "members.invitation.acceptError")
    } finally {
      setBusy(false)
    }
  }

  const panelTitle = i18n("members.invitation.title")

  return (
    <AuthPageFrame>
      <AuthActionPanel title={panelTitle}>
        {!token ? <p className={`${styles.state} text-body`}>{i18n("members.invitation.invalid")}</p> : previewQuery.isPending ? (
          <p className={`${styles.state} text-body`}>{i18n("members.invitation.loading")}</p>
        ) : invalid || !invitation ? (
          <p className={`${styles.state} text-body`} role="alert">{i18n("members.invitation.invalid")}</p>
        ) : hasSession && userQuery.isPending ? (
          <p className={`${styles.state} text-body`}>{i18n("members.invitation.loading")}</p>
        ) : (
          <div className={styles.content}>
            <p className={`${styles.project} text-body`}>{i18n("members.invitation.project", { project: invitation.project_title ?? "" })}</p>
            <p className={`${styles.role} text-body`}>{i18n("members.invitation.role", { role: i18n(`members.role.${invitation.role_id ?? "viewer"}` as MessageKey) })}</p>
            {hasSession && userQuery.isError ? (
              <p className={styles.state} role="alert">{i18n("members.error.generic")}</p>
            ) : (
              <InvitationAcceptForm
                email={email}
                mode={mode}
                signedIn={signedIn}
                accountMismatch={accountMismatch}
                busy={busy}
                error={error}
                tokenHash={location.hash}
                onAcceptWithPassword={(password) => void accept(password)}
                onLogin={(password) => void loginAndAccept(password)}
                onAcceptSignedIn={() => void accept()}
                onSwitchAccount={() => void clearSession()}
              />
            )}
          </div>
        )}
      </AuthActionPanel>
    </AuthPageFrame>
  )
}
