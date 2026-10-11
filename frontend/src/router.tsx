import { createBrowserRouter, Navigate, Outlet, replace } from "react-router"
import App from "./App"
import { DEFAULT_TAB } from "./store/tab"
import { getPersonalAccessToken } from "~/features/auth/lib/localStorage"
import PageHeading from "~/features/shared/components/PageHeading"
import LoginPage from "./pages/Login"
import SignupPage from "./pages/Signup"
import ProjectsNewPage from "./pages/ProjectsNew"
import ProjectsEditPage from "./pages/ProjectsEdit"
import ProjectList from "~/pages/ProjectList"
import TaskList from "~/pages/TaskList"
import TaskNewPage from "~/pages/TaskNew"
import TaskEditPage from "~/pages/TaskEdit"
import ScheduleList from "~/pages/ScheduleList"
import ScheduleNewPage from "~/pages/ScheduleNew"
import ScheduleEditPage from "~/pages/ScheduleEdit"
import InboxPage from "~/pages/Inbox"

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
      { path: "/inbox", element: <App><InboxPage /></App> },
      { path: "/projects", element: <App><ProjectList /></App> },
      { path: "/projects/new", element: <App><ProjectsNewPage /></App> },
      { path: "/projects/:id/edit", element: <App><ProjectsEditPage /></App> },
      { path: "/tasks", element: <App><TaskList /></App> },
      { path: "/tasks/new", element: <App><TaskNewPage /></App> },
      { path: "/tasks/:id/edit", element: <App><TaskEditPage /></App> },
      { path: "/schedules", element: <App><ScheduleList /></App> },
      { path: "/schedules/new", element: <App><ScheduleNewPage /></App> },
      { path: "/schedules/:blueprintId/:occurrenceDate/edit", element: <App><ScheduleEditPage /></App> },
      { path: "/members", element: <App><PageHeading messageKey="page.members.title" /></App> },
      { path: "/kpi", element: <App><PageHeading messageKey="page.kpi.title" /></App> },
      { path: "/profile", element: <App><PageHeading messageKey="page.profile.title" /></App> },
      { path: "*", element: <App><PageHeading messageKey="page.notFound.title" /></App> },
    ],
  },
])

if (typeof window !== "undefined") {
  window.addEventListener("auth:unauthorized", () => {
    void router.navigate("/login", { replace: true })
  })
}
