import { useI18n, useLanguage } from "~/features/i18n/hooks"
import Switcher from "~/features/shared/components/Switcher"

export default function LanguageSwitcher() {
  const i18n = useI18n()
  const { language, setLanguage } = useLanguage()

  return (
    <Switcher
      label={i18n("common.language")}
      leftLabel={i18n("common.language.en")}
      rightLabel={i18n("common.language.ja")}
      left="EN"
      right="JP"
      rightSelected={language === "ja"}
      onChange={(rightSelected) => setLanguage(rightSelected ? "ja" : "en")}
    />
  )
}
