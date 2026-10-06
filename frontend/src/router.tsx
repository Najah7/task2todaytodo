import { createBrowserRouter, Navigate } from "react-router"
import App from "./App"
import { DEFAULT_TAB } from "./store/tab"
import PageHeading from "~/features/shared/components/PageHeading"
import LoginPage from "./pages/Login"
import SignupPage from "./pages/Signup"

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/signup", element: <SignupPage /> },
  { path: "/", element: <Navigate to={`/${DEFAULT_TAB}`} replace /> },
  { path: "/today", element: <App><PageHeading messageKey="page.today.title" /></App> },
  { path: "/inbox", element: <App><PageHeading messageKey="page.inbox.title" /></App> },
  { path: "/projects", element: <App><PageHeading messageKey="page.projects.title" /></App> },
  { path: "/calendar", element: <App><PageHeading messageKey="page.calendar.title" /></App> },
  { path: "/kpi", element: <App><PageHeading messageKey="page.kpi.title" /></App> },
  { path: "/profile", element: <App><PageHeading messageKey="page.profile.title" /></App> },
  { path: "*", element: <App><PageHeading messageKey="page.notFound.title" /></App> },
])
