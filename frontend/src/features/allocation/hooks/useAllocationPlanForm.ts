import { useContext, useRef, useState } from "react"
import { useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { ApiError } from "~/api/http"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useI18n } from "~/features/i18n/hooks"
import { useLanguage } from "~/features/i18n/hooks"
import { allocationErrorMessage } from "~/features/allocation/lib/errorMessage"
import {
  allocationFormValues2planConfig,
  allocationFormValues2planRequests,
  allocationFormValues2ruleConfig,
  allocationFormValues2ruleRequests,
  allocationResources2formValues,
  allocationRulesResponse2identities,
} from "~/features/allocation/converters/allocationResources2formValues"
import { AllocationAPIDataContext } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { AllocationActionContext } from "~/features/allocation/providers/AllocationActionProvider/context"
import { AllocationNavigationContext } from "~/features/allocation/providers/AllocationNavigationProvider/context"
import { allocationPlanSchema, type AllocationPlanValues } from "~/features/allocation/components/AllocationPlanForm/schema"

type SaveErrors = { rules?: string; plans?: string }

export function useAllocationPlanForm() {
  const data = useContext(AllocationAPIDataContext)!
  const actions = useContext(AllocationActionContext)!
  const navigation = useContext(AllocationNavigationContext)!
  const i18n = useI18n()
  const { language } = useLanguage()
  const initialRules = useRef(data.rules!).current
  const initialPlans = useRef(data.plans!).current
  const initialTimezone = useRef(data.timezone).current
  const initialValues = allocationResources2formValues([initialRules], [initialPlans])[0]!
  const baselineRules = useRef(allocationFormValues2ruleConfig([initialValues], initialTimezone)[0]!)
  const baselinePlans = useRef(allocationFormValues2planConfig([initialValues])[0]!)
  const baselineRevisions = useRef({ rules: initialRules.revision!, plans: initialPlans.revision! }).current
  const form = useForm<AllocationPlanValues>({ resolver: zodResolver(allocationPlanSchema), defaultValues: initialValues })
  const values = useWatch({ control: form.control }) as AllocationPlanValues
  const currentRuleConfig = allocationFormValues2ruleConfig([values], initialTimezone)[0]!
  const currentPlanConfig = allocationFormValues2planConfig([values])[0]!
  const rulesDirty = JSON.stringify(currentRuleConfig) !== JSON.stringify(baselineRules.current)
  const plansDirty = JSON.stringify(currentPlanConfig) !== JSON.stringify(baselinePlans.current)
  const dirtyGuard = useFormDraftGuard(rulesDirty || plansDirty)
  const [saveErrors, setSaveErrors] = useState<SaveErrors>({})
  const [savedMessage, setSavedMessage] = useState<string>()

  function errorText(error: unknown) {
    return allocationErrorMessage(error, i18n, {
      status: error instanceof ApiError ? error.status : undefined,
      timezone: initialTimezone,
      language,
      projectNames: new Map([
        ...(data.plans?.items ?? []).filter((project) => project.project_id && project.title).map((project) => [project.project_id!, project.title!] as const),
        ...(data.rules?.items ?? []).filter((rule) => rule.project_id && rule.project_title).map((rule) => [rule.project_id!, rule.project_title!] as const),
        ...data.projects.filter((project) => project.id && project.title).map((project) => [project.id!, project.title!] as const),
      ]),
    })
  }

  async function submit(nextValues: AllocationPlanValues) {
    const saveRules = JSON.stringify(allocationFormValues2ruleConfig([nextValues], initialTimezone)[0]) !== JSON.stringify(baselineRules.current)
    const savePlans = JSON.stringify(allocationFormValues2planConfig([nextValues])[0]) !== JSON.stringify(baselinePlans.current)
    if (!saveRules && !savePlans) {
      dirtyGuard.markClean()
      navigation.backToPlan()
      return
    }

    setSaveErrors({})
    setSavedMessage(undefined)
    let result: Awaited<ReturnType<typeof actions.savePlanParts>>
    try {
      result = await actions.savePlanParts({
        revisions: baselineRevisions,
        ...(saveRules ? { rules: { items: allocationFormValues2ruleRequests([nextValues], initialTimezone)[0] } } : {}),
        ...(savePlans ? { plans: allocationFormValues2planRequests([nextValues])[0] } : {}),
      })
    } catch (error) {
      setSaveErrors({
        ...(saveRules ? { rules: errorText(error) } : {}),
        ...(savePlans ? { plans: errorText(error) } : {}),
      })
      return
    }

    const identities = result.rules ? allocationRulesResponse2identities([result.rules], [nextValues])[0]! : nextValues
    if (result.rules) {
      baselineRules.current = allocationFormValues2ruleConfig([identities], initialTimezone)[0]!
      form.setValue("projects", identities.projects, { shouldDirty: false })
    }
    if (result.plans) baselinePlans.current = allocationFormValues2planConfig([nextValues])[0]!

    const errors: SaveErrors = {}
    if ("rules" in result.errors) errors.rules = errorText(result.errors.rules)
    if ("plans" in result.errors) errors.plans = errorText(result.errors.plans)
    setSaveErrors(errors)

    if (Object.keys(errors).length > 0) {
      const saved = [result.rules && i18n("weeklyPlan.plan.rulesSaved"), result.plans && i18n("weeklyPlan.plan.plansSaved")].filter(Boolean).join(" ")
      setSavedMessage(saved || undefined)
      return
    }

    baselineRules.current = allocationFormValues2ruleConfig([identities], initialTimezone)[0]!
    baselinePlans.current = allocationFormValues2planConfig([nextValues])[0]!
    form.reset(identities)
    dirtyGuard.markClean()
    navigation.backToPlan()
  }

  return {
    form,
    viewProps: {
      projects: data.projects,
      projectTitles: Object.fromEntries([
        ...(data.plans?.items ?? []).filter((project) => project.project_id && project.title).map((project) => [project.project_id!, project.title!] as const),
        ...(data.rules?.items ?? []).filter((rule) => rule.project_id && rule.project_title).map((rule) => [rule.project_id!, rule.project_title!] as const),
      ]),
      timezone: initialTimezone,
      busy: actions.busy,
      onSubmit: form.handleSubmit(submit),
      onCancel: () => navigation.backToPlan(),
      discardConfirmation: {
        open: dirtyGuard.dialogOpen,
        onConfirm: dirtyGuard.leave,
        onCancel: dirtyGuard.stay,
      },
      errors: saveErrors,
      savedMessage,
    },
  }
}
