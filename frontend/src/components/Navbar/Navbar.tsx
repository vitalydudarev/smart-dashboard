import { MapPin } from 'lucide-react'
import ThemeToggle from '../ThemeToggle/ThemeToggle.tsx'
import './Navbar.scss'

function Navbar() {
  return (
    <header className="header">
      <div className="header-inner">
        <span className="brand">
          <MapPin className="brand-mark" size={18} />
          <span>Smart Dashboard</span>
        </span>

        <div className="header-end">
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}

export default Navbar
