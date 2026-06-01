import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    proxy: {
      // Forward REST API calls to aom serve (default port 7777)
      '/api': {
        target: 'http://localhost:7777',
        changeOrigin: true,
      },
      // Forward WebSocket connections to aom serve
      '/ws': {
        target: 'ws://localhost:7777',
        ws: true,
      },
    },
  },
})
