import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import styles from "./index.module.css"

type PageHeadingProps = { messageKey: MessageKey; id?: string; descriptionKey?: MessageKey }

export default function PageHeading({ messageKey, id, descriptionKey }: PageHeadingProps) {
  const i18n = useI18n()
  return (
    <div className={styles.pageHeading}>
      <h1 className="text-page-title" id={id}>{i18n(messageKey)}</h1>
      {descriptionKey && <p className="text-description">{i18n(descriptionKey)}</p>}
    </div>
  )
}
