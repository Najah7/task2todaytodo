import { http, HttpResponse } from "msw"

export const authHandlers = {
  login: http.post("*/api/login", () => HttpResponse.json({ token: "storybook-personal-access-token" })),
  signup: http.post("*/api/signup", () => HttpResponse.json({ user_id: "storybook-user" }, { status: 201 })),
}
