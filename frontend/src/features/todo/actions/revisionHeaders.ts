export function todoRevisionHeaders(revision: number): Record<string, string> {
  return { "If-Match": `"${revision}"` }
}
