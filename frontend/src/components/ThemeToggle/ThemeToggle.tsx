import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Monitor, Moon, Sun, type LucideIcon } from 'lucide-react'
import { useThemePreference, type ThemePreference } from '../../theme/useThemePreference.ts'
import './ThemeToggle.scss'

const options: readonly { value: ThemePreference; label: string; icon: LucideIcon }[] = [
  { value: 'system', label: 'Auto (match system)', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
]

/**
 * Theme switcher: auto (follow the OS), light, dark. Shows only the current choice's icon; clicking
 * it drops the three options down in a vertical column, which keeps the navbar narrow on phones.
 */
function ThemeToggle() {
  const [preference, setPreference] = useThemePreference()
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const optionRefs = useRef<(HTMLButtonElement | null)[]>([])

  const current = options.find((option) => option.value === preference) ?? options[0]
  const CurrentIcon = current.icon

  // Close on a click or tap anywhere outside the switcher.
  useEffect(() => {
    if (!open) return

    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }

    document.addEventListener('pointerdown', onPointerDown)

    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  // Move focus to the selected option when the menu opens, so keyboard users land on it.
  useEffect(() => {
    if (open) optionRefs.current[options.indexOf(current)]?.focus()
  }, [open, current])

  const close = () => {
    setOpen(false)
    triggerRef.current?.focus()
  }

  const choose = (value: ThemePreference) => {
    setPreference(value)
    close()
  }

  const onMenuKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const focused = optionRefs.current.indexOf(document.activeElement as HTMLButtonElement)

    if (event.key === 'Escape') {
      event.preventDefault()
      close()
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const step = event.key === 'ArrowDown' ? 1 : -1
      optionRefs.current[(focused + step + options.length) % options.length]?.focus()
    } else if (event.key === 'Tab') {
      setOpen(false)
    }
  }

  return (
    <div className="theme-toggle" ref={rootRef}>
      <div className="segmented">
        <button
          ref={triggerRef}
          type="button"
          className="segment segment--selected"
          aria-haspopup="menu"
          aria-expanded={open}
          aria-label={`Colour theme: ${current.label}`}
          title={`Colour theme: ${current.label}`}
          onClick={() => setOpen((isOpen) => !isOpen)}
        >
          <CurrentIcon size={18} />
        </button>
      </div>

      {open && (
        <div
          className="segmented segmented--vertical theme-toggle-menu"
          role="menu"
          aria-label="Colour theme"
          onKeyDown={onMenuKeyDown}
        >
          {options.map(({ value, label, icon: Icon }, index) => (
            <button
              key={value}
              ref={(element) => {
                optionRefs.current[index] = element
              }}
              type="button"
              role="menuitemradio"
              className={value === preference ? 'segment segment--selected' : 'segment'}
              aria-checked={value === preference}
              aria-label={label}
              title={label}
              onClick={() => choose(value)}
            >
              <Icon size={18} />
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export default ThemeToggle
