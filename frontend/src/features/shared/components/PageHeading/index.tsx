import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import styles from "~/App.module.css"

export default function PageHeading({ messageKey }: { messageKey: MessageKey }) {
  const i18n = useI18n()
  return <h1 className={`${styles.pageTitle} text-page-title`}>{i18n(messageKey)}</h1>
}
