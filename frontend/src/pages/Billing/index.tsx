import { useQueryClient } from "@tanstack/react-query"
import { Link, useSearchParams } from "react-router"
import { changeBillingPlan, createBillingPortalSession, getGetBillingStatusQueryKey, startBillingCheckout, useGetBillingStatus } from "~/api/generated/billing"
import { useI18n } from "~/features/i18n/hooks"
import type { MessageKey } from "~/features/i18n/messages/types"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const plans = [
  { id: "free", name: "billing.plan.free", price: "¥0", detail: "billing.detail.free" },
  { id: "pro", name: "billing.plan.pro", price: "¥1,350", detail: "billing.detail.pro" },
  { id: "team", name: "billing.plan.team", price: "¥2,250", detail: "billing.detail.team" },
] as const
const statusMessages: Record<string, MessageKey> = {
  none: "billing.status.none",
  active: "billing.status.active",
  trialing: "billing.status.trialing",
  past_due: "billing.status.past_due",
  unpaid: "billing.status.unpaid",
  paused: "billing.status.paused",
  incomplete: "billing.status.incomplete",
  canceled: "billing.status.canceled",
}

function isStripeURL(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === "https:" && ["checkout.stripe.com", "billing.stripe.com"].includes(url.hostname)
  } catch {
    return false
  }
}

function paidStatus(status?: string) {
  return ["active", "trialing", "past_due", "unpaid", "paused"].includes(status ?? "")
}

export default function BillingPage() {
  const i18n = useI18n()
  const [searchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const state = useGetBillingStatus({ query: { retry: false, refetchOnWindowFocus: true } })
  const paid = paidStatus(state.data?.status)
  const incomplete = state.data?.status === "incomplete"
  const hasSubscription = paid || incomplete
  const activePlan = paid ? state.data?.plan ?? "free" : "free"
  const busy = state.isPending

  const refresh = () => queryClient.invalidateQueries({ queryKey: getGetBillingStatusQueryKey() })
  const startCheckout = async (plan: "pro" | "team") => {
    try {
      const result = await startBillingCheckout({ plan })
      if (!result.url || !isStripeURL(result.url)) throw new Error("invalid Stripe Checkout URL")
      window.location.assign(result.url)
    } catch {
      notify.error(i18n("billing.action.error"))
    }
  }
  const changePlan = async (plan: "pro" | "team") => {
    try {
      await changeBillingPlan({ plan })
      await refresh()
      notify.success(i18n("billing.plan.updated"))
    } catch {
      notify.error(i18n("billing.action.error"))
      await refresh()
    }
  }
  const openPortal = async () => {
    try {
      const result = await createBillingPortalSession()
      if (!result.url || !isStripeURL(result.url)) throw new Error("invalid Stripe Portal URL")
      window.location.assign(result.url)
    } catch {
      notify.error(i18n("billing.action.error"))
    }
  }

  const checkoutResult = searchParams.get("checkout")
  const pendingPlan = state.data?.pending_plan
  const pendingAt = state.data?.pending_plan_at
  const statusKey = statusMessages[state.data?.status ?? "none"] ?? "billing.status.none"

  if (state.isPending && !state.data) return <p className="text-body">{i18n("billing.loading")}</p>
  if (state.isError && !state.data) return <div className={styles.error} role="alert">
    <p className="text-body">{i18n("billing.loadError")}</p>
    <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => { void state.refetch() }}>{i18n("billing.retry")}</button>
  </div>

  return <section className={styles.page} aria-labelledby="billing-title">
    <header className={styles.heading}>
      <div>
        <p className={`${styles.eyebrow} text-caption`}>{i18n("billing.eyebrow")}</p>
        <h1 className="text-page-title" id="billing-title">{i18n("billing.title")}</h1>
        <p className={`${styles.description} text-body`}>{i18n("billing.description")}</p>
      </div>
      <Link className={`${styles.close} ${controls.focusRing}`} to="/today" aria-label={i18n("common.close")}>×</Link>
    </header>

    {checkoutResult === "success" && <p className={styles.notice} role="status">{i18n("billing.checkout.submitted")}</p>}
    {checkoutResult === "cancelled" && <p className={styles.notice} role="status">{i18n("billing.checkout.cancelled")}</p>}
    {state.isError && <p className={styles.notice} role="status">{i18n("billing.refreshError")}</p>}

    <div className={styles.planGrid}>
      {plans.map((plan) => {
        const isCurrent = activePlan === plan.id
        const isPro = plan.id === "pro"
        const isPaidCard = plan.id !== "free"
        const isPending = pendingPlan === plan.id
        const isAwaitingPayment = incomplete && state.data?.plan === plan.id
        return <article className={`${styles.planCard} ${isPro ? styles.featured : ""}`} key={plan.id}>
          {isPro && <span className={styles.recommended}>{i18n("billing.recommended")}</span>}
          <div className={styles.planTitleRow}>
            <h2 className="text-card-title">{i18n(plan.name)}</h2>
            {isCurrent && <span className={styles.currentTag}>{i18n("billing.current")}</span>}
            {isAwaitingPayment && <span className={styles.currentTag}>{i18n("billing.awaitingPayment")}</span>}
          </div>
          <p className={styles.price}><strong>{plan.price}</strong><span className="text-caption">{isPaidCard ? i18n("billing.perUserMonth") : i18n("billing.freeForever")}</span></p>
          <p className={`${styles.detail} text-body`}>{i18n(plan.detail)}</p>
          {isCurrent && paid && <p className={`${styles.status} text-caption`}>{i18n(statusKey)}</p>}
          {isAwaitingPayment && <p className={`${styles.status} text-caption`}>{i18n("billing.status.incomplete")}</p>}
          {isCurrent && state.data?.cancel_at_period_end && <p className={`${styles.status} text-caption`}>{i18n("billing.cancelsAt", { date: state.data.current_period_end ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(new Date(state.data.current_period_end)) : "" })}</p>}
          {isPending && <p className={`${styles.status} text-caption`}>{i18n(pendingAt ? "billing.plan.pendingAt" : "billing.plan.pendingPayment", { date: pendingAt ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(new Date(pendingAt)) : "" })}</p>}
          <div className={styles.cardFooter}>
            {plan.id === "free" && !paid && <span className={styles.currentMessage}>{i18n("billing.free.current")}</span>}
            {plan.id === "free" && hasSubscription && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} disabled={busy} type="button" onClick={() => { void openPortal() }}>{i18n("billing.manage")}</button>}
            {isPaidCard && !hasSubscription && <button className={`${controls.button} ${isPro ? controls.primaryButton : controls.outlineButton} ${controls.focusRing} text-button-small`} disabled={busy} type="button" onClick={() => { void startCheckout(plan.id) }}>{i18n("billing.subscribe")}</button>}
            {isPaidCard && paid && isCurrent && <span className={styles.currentMessage}>{i18n("billing.current")}</span>}
            {isPaidCard && paid && isCurrent && Boolean(pendingAt) && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} disabled={busy} type="button" onClick={() => { void changePlan(plan.id) }}>{i18n("billing.keepCurrent")}</button>}
            {isPaidCard && paid && !isCurrent && <button className={`${controls.button} ${isPro ? controls.primaryButton : controls.outlineButton} ${controls.focusRing} text-button-small`} disabled={busy || Boolean(pendingPlan) || state.data?.status !== "active" || state.data.cancel_at_period_end} type="button" onClick={() => { void changePlan(plan.id) }}>{i18n(plan.id === "team" ? "billing.changeToTeam" : "billing.changeToPro")}</button>}
          </div>
        </article>
      })}
    </div>

    {hasSubscription && <div className={styles.manage}>
      <div><h2 className="text-card-title">{i18n("billing.manageTitle")}</h2><p className="text-body">{i18n("billing.manageDescription")}</p></div>
      <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy} onClick={() => { void openPortal() }}>{i18n("billing.openPortal")}</button>
    </div>}
    {pendingPlan && <p className={`${styles.pendingNote} text-caption`}>{i18n(pendingAt ? "billing.pendingNote" : "billing.paymentNote")}</p>}
  </section>
}
