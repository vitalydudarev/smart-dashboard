import { Route, Routes } from 'react-router'
import FootballWidget from './components/FootballWidget/FootballWidget.tsx'
import Navbar from './components/Navbar/Navbar.tsx'
import RateWidget from './components/RateWidget/RateWidget.tsx'
import WeatherWidget from './components/WeatherWidget/WeatherWidget.tsx'
import FootballPage from './pages/FootballPage/FootballPage.tsx'
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
                <FootballWidget />
              </div>
            }
          />
          <Route path="rates" element={<RatesPage />} />
          <Route path="weather" element={<WeatherPage />} />
          <Route path="football" element={<FootballPage />} />
        </Routes>
      </main>
    </>
  )
}

export default App
