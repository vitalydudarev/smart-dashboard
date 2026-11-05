import { Monitor, Moon, Sun, type LucideIcon } from 'lucide-react'
import { useThemePreference, type ThemePreference } from '../../theme/useThemePreference.ts'

const options: readonly { value: ThemePreference; label: string; icon: LucideIcon }[] = [
  { value: 'system', label: 'Auto (match system)', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
]

/** Three-way theme switcher: auto (follow the OS), light, dark. */
function ThemeToggle() {
  const [preference, setPreference] = useThemePreference()

  return (
    <div className="segmented" role="group" aria-label="Colour theme">
      {options.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          className={value === preference ? 'segment segment--selected' : 'segment'}
          aria-pressed={value === preference}
          aria-label={label}
          title={label}
          onClick={() => setPreference(value)}
        >
          <Icon size={18} />
        </button>
      ))}
    </div>
  )
}

export default ThemeToggle
