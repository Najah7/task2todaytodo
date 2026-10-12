import { expect, test } from "vitest"
import { todoRevisionHeaders } from "./revisionHeaders"

test("uses the quoted ETag required by If-Match", () => {
  expect(todoRevisionHeaders(1)).toEqual({ "If-Match": '"1"' })
})
