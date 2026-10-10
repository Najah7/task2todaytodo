import controls from "~/styles/controls.module.css"

type Props = {
  label: string
  onClick: () => void
}

export default function ProjectCreateButton({ label, onClick }: Props) {
  return (
    <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" onClick={onClick}>
      {label}
    </button>
  )
}
