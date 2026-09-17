import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: { allowedHosts: ['jz.wangj.de'], host: '0.0.0.0', port: 5173, proxy: { '/api': 'http://localhost:8080', '/healthz': 'http://localhost:8080' } },
})
