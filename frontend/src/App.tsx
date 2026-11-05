import { Route, Routes } from 'react-router'
import Navbar from './components/Navbar/Navbar.tsx'
import RateWidget from './components/RateWidget/RateWidget.tsx'
import RatesPage from './pages/RatesPage/RatesPage.tsx'

function App() {
  return (
    <>
      <Navbar />
      <main className="content">
        <Routes>
          <Route index element={<RateWidget />} />
          <Route path="rates" element={<RatesPage />} />
        </Routes>
      </main>
    </>
  )
}

export default App
