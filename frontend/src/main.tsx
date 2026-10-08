import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { RouterProvider } from 'react-router/dom'
import { router } from './router.tsx'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LanguageProvider } from '~/features/i18n'
import { initializeDisplay } from '~/features/display'
import { NotificationViewport } from '~/features/shared/notification'

const queryClient = new QueryClient()

initializeDisplay()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <LanguageProvider>
        <RouterProvider router={router} />
        <NotificationViewport />
      </LanguageProvider>
    </QueryClientProvider>
  </StrictMode>,
)
