/* oxlint-disable react/only-export-components -- Storybook decorators wrap preview-only components. */
import { useState, type ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router"
import type { Decorator } from "@storybook/react-vite"
import { useI18n } from "~/features/i18n/hooks"
import { NotificationViewport } from "~/features/shared/notification"
import styles from "./index.module.css"

function AuthFormPreview({ children }: { children: ReactNode }) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { mutations: { retry: false } } }))
  const i18n = useI18n()
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <div className={styles.frame}>
          <Routes>
            <Route path="/" element={children} />
            <Route path="/today" element={<h1 className="text-page-title">{i18n("page.today.title")}</h1>} />
          </Routes>
          <NotificationViewport />
        </div>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

export const withAuthForm: Decorator = (Story, context) => (
  <AuthFormPreview key={context.id}><Story /></AuthFormPreview>
)

export const withFieldWidth: Decorator = (Story) => <div className={styles.frame}><Story /></div>
