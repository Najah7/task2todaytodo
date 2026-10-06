/* oxlint-disable react/only-export-components -- Storybook configuration exports decorators. */
import type { Preview, ReactRenderer } from "@storybook/react-vite"
import { withThemeByDataAttribute } from "@storybook/addon-themes"
import { useGlobals } from "storybook/preview-api"
import { mswLoader } from "msw-storybook-addon/csf3"
import { setupWorker } from "msw/browser"
import { LanguageProviderContext } from "~/features/i18n/languageContext"
import "~/index.css"

const preview: Preview = {
  tags: ["autodocs"],
  parameters: { layout: "centered" },
  globalTypes: {
    locale: {
      description: "Display language",
      toolbar: {
        icon: "globe",
        items: [{ value: "ja", title: "日本語" }, { value: "en", title: "English" }],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { locale: "ja" },
  loaders: [mswLoader(async () => {
    const worker = setupWorker()
    await worker.start({
      quiet: true,
      onUnhandledRequest(request, print) {
        if (new URL(request.url).pathname.startsWith("/api/")) print.error()
      },
    })
    return worker
  })],
  decorators: [
    function WithLanguage(Story) {
      const [globals, updateGlobals] = useGlobals()
      const language = globals.locale === "en" ? "en" : "ja"
      return (
        <LanguageProviderContext.Provider value={{ language, setLanguage: (locale) => updateGlobals({ locale }) }}>
          <div lang={language}><Story /></div>
        </LanguageProviderContext.Provider>
      )
    },
    withThemeByDataAttribute<ReactRenderer>({
      themes: { light: "light", dark: "dark" },
      defaultTheme: "light",
      attributeName: "data-display",
    }),
  ],
}

export default preview
