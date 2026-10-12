import { createBrowserRouter, Navigate, Outlet, replace } from "react-router"
import App from "./App"
import { DEFAULT_TAB } from "./store/tab"
import { getPersonalAccessToken } from "~/features/auth/lib/localStorage"
import PageHeading from "~/features/shared/components/PageHeading"
import LoginPage from "./pages/Login"
import SignupPage from "./pages/Signup"
import VerifyEmailPage from "./pages/VerifyEmail"
import RequestPasswordResetPage from "./pages/RequestPasswordReset"
import ResetPasswordPage from "./pages/ResetPassword"
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
import WeeklyPlanPage from "~/pages/WeeklyPlan"
import WeeklyPlanEditPage from "~/pages/WeeklyPlanEdit"
import ProfileSleepSettings from "~/pages/ProfileSleep"
import TodayPage from "~/pages/Today"

function requireAuthentication() {
  if (!getPersonalAccessToken()?.trim()) return replace("/login")
  return null
}

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/signup", element: <SignupPage /> },
  { path: "/verify-email", element: <VerifyEmailPage /> },
  { path: "/request-password-reset", element: <RequestPasswordResetPage /> },
  { path: "/reset-password", element: <ResetPasswordPage /> },
  {
    loader: requireAuthentication,
    shouldRevalidate: () => true,
    element: <Outlet />,
    children: [
      { path: "/", element: <Navigate to={`/${DEFAULT_TAB}`} replace /> },
      { path: "/today", element: <App><TodayPage /></App> },
      { path: "/inbox", element: <App><InboxPage /></App> },
      { path: "/projects", element: <App><ProjectList /></App> },
      { path: "/projects/new", element: <App><ProjectsNewPage /></App> },
      { path: "/projects/:id/edit", element: <App><ProjectsEditPage /></App> },
      { path: "/tasks", element: <App><TaskList /></App> },
      { path: "/tasks/new", element: <App><TaskNewPage /></App> },
      { path: "/tasks/:id/edit", element: <App><TaskEditPage /></App> },
      { path: "/schedules", element: <App><ScheduleList /></App> },
      { path: "/schedules/new", element: <App><ScheduleNewPage /></App> },
      { path: "/schedules/:ruleId/:occurrenceDate/edit", element: <App><ScheduleEditPage /></App> },
      { path: "/weekly-plan", element: <App><WeeklyPlanPage /></App> },
      { path: "/weekly-plan/edit", element: <App><WeeklyPlanEditPage /></App> },
      { path: "/members", element: <App><PageHeading messageKey="page.members.title" /></App> },
      { path: "/kpi", element: <App><PageHeading messageKey="page.kpi.title" /></App> },
      { path: "/profile", element: <App><ProfileSleepSettings /></App> },
      { path: "*", element: <App><PageHeading messageKey="page.notFound.title" /></App> },
    ],
  },
])

if (typeof window !== "undefined") {
  window.addEventListener("auth:unauthorized", () => {
    void router.navigate("/login", { replace: true })
  })
}
