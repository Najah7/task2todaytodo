import { createBrowserRouter, Navigate, Outlet, replace } from "react-router"
import App from "./App"
import { DEFAULT_TAB } from "./store/tab"
import { getPersonalAccessToken } from "~/features/auth/lib/localStorage"
import PageHeading from "~/features/shared/components/PageHeading"
import LoginPage from "./pages/Login"
import SignupPage from "./pages/Signup"
import ProjectsNewPage from "./pages/ProjectsNew"
import ProjectsEditPage from "./pages/ProjectsEdit"
import ProjectList from "~/features/project/components/ProjectList"

function requireAuthentication() {
  if (!getPersonalAccessToken()?.trim()) return replace("/login")
  return null
}

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/signup", element: <SignupPage /> },
  {
    loader: requireAuthentication,
    shouldRevalidate: () => true,
    element: <Outlet />,
    children: [
      { path: "/", element: <Navigate to={`/${DEFAULT_TAB}`} replace /> },
      { path: "/today", element: <App><PageHeading messageKey="page.today.title" /></App> },
      { path: "/inbox", element: <App><PageHeading messageKey="page.inbox.title" /></App> },
      { path: "/projects", element: <App><ProjectList /></App> },
      { path: "/projects/new", element: <App><ProjectsNewPage /></App> },
      { path: "/projects/:id/edit", element: <App><ProjectsEditPage /></App> },
      { path: "/tasks", element: <App><PageHeading messageKey="page.tasks.title" /></App> },
      { path: "/calendar", element: <App><PageHeading messageKey="page.calendar.title" /></App> },
      { path: "/members", element: <App><PageHeading messageKey="page.members.title" /></App> },
      { path: "/kpi", element: <App><PageHeading messageKey="page.kpi.title" /></App> },
      { path: "/profile", element: <App><PageHeading messageKey="page.profile.title" /></App> },
      { path: "*", element: <App><PageHeading messageKey="page.notFound.title" /></App> },
    ],
  },
])
