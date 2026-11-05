import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Forward API calls to the Go backend (see backend/main.go).
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
