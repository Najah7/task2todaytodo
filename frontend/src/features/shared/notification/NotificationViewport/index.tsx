import { useAtomValue } from "jotai"
import { Toaster } from "sonner"
import { useI18n } from "~/features/i18n/hooks"
import { modeAtom } from "~/store/mode"
import styles from "./index.module.css"

export default function NotificationViewport() {
  const mode = useAtomValue(modeAtom)
  const i18n = useI18n()

  return (
    <Toaster
      className={styles.viewport}
      theme={mode}
      position="bottom-right"
      offset={16}
      mobileOffset={16}
      visibleToasts={3}
      closeButton
      richColors
      containerAriaLabel={i18n("common.notifications")}
    />
  )
}
