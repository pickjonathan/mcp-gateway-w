import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev, proxy the control-plane API and the metrics query API so the browser
// stays same-origin (no CORS needed locally). Targets come from env.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/v1': {
        target: process.env.VITE_API_BASE || 'http://localhost:8090',
        changeOrigin: true,
      },
      '/metrics-api': {
        target: process.env.VITE_METRICS_BASE || 'http://localhost:9090',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/metrics-api/, ''),
      },
    },
  },
  build: { outDir: 'dist', sourcemap: true },
})
