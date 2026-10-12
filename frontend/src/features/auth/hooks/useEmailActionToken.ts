import { useLocation } from "react-router"

export function useEmailActionToken() {
  const location = useLocation()
  const token = new URLSearchParams(location.hash.slice(1)).get("token")
  return token
}
