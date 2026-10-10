import type { RestProjectPriorityResponse, RestProjectTypeResponse } from "~/api/generated/projects"
import type { Language } from "~/features/i18n/types"

type ProjectOption = RestProjectTypeResponse | RestProjectPriorityResponse

export function projectOptions2projectFormOptions(options: ProjectOption[], language: Language) {
  return options.map((option) => ({
    value: option.value,
    label: language === "ja" ? option.label_jp : option.label,
  }))
}
