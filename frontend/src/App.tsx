import { Route, Routes } from 'react-router'
import Navbar from './components/Navbar/Navbar.tsx'
import RateWidget from './components/RateWidget/RateWidget.tsx'
import WeatherWidget from './components/WeatherWidget/WeatherWidget.tsx'
import RatesPage from './pages/RatesPage/RatesPage.tsx'
import WeatherPage from './pages/WeatherPage/WeatherPage.tsx'

function App() {
  return (
    <>
      <Navbar />
      <main className="content">
        <Routes>
          <Route
            index
            element={
              <div className="widgets">
                <RateWidget />
                <WeatherWidget />
              </div>
            }
          />
          <Route path="rates" element={<RatesPage />} />
          <Route path="weather" element={<WeatherPage />} />
        </Routes>
      </main>
    </>
  )
}

export default App
