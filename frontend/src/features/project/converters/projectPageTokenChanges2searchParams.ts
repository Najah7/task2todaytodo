type ProjectPageTokenChange = { searchParams: URLSearchParams; pageToken?: string }

export function projectPageTokenChanges2searchParams(changes: ProjectPageTokenChange[]): URLSearchParams[] {
  return changes.map(({ searchParams, pageToken }) => {
    const next = new URLSearchParams(searchParams)
    if (pageToken) next.set("page_token", pageToken)
    else next.delete("page_token")
    return next
  })
}
