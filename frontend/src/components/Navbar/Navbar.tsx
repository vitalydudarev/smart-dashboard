import { MapPin } from 'lucide-react'
import { Link } from 'react-router'
import ThemeToggle from '../ThemeToggle/ThemeToggle.tsx'
import './Navbar.scss'

function Navbar() {
  return (
    <header className="header">
      <div className="header-inner">
        <Link className="brand" to="/">
          <MapPin className="brand-mark" size={18} />
          <span>Smart Dashboard</span>
        </Link>

        <div className="header-end">
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}

export default Navbar
