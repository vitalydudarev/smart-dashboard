import Navbar from './components/Navbar/Navbar.tsx'
import RateWidget from './components/RateWidget/RateWidget.tsx'

function App() {
  return (
    <>
      <Navbar />
      <main className="content">
        <RateWidget />
      </main>
    </>
  )
}

export default App
