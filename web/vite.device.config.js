import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const root = path.dirname(fileURLToPath(import.meta.url))
export default defineConfig({
  base: '/', plugins: [react()], resolve: { alias: { '@': path.resolve(root, 'src') } },
  build: { outDir: 'device-dist', rollupOptions: { input: path.resolve(root, 'device.html') } },
  server: { proxy: { '/v1': 'http://127.0.0.1:8788' } },
})
