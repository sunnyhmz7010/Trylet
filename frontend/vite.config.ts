import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: './',
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { host: '127.0.0.1', strictPort: true },
  build: {
    rollupOptions: {
      input: {
        editor: fileURLToPath(new URL('./index.html', import.meta.url)),
        ball: fileURLToPath(new URL('./ball.html', import.meta.url)),
      },
    },
  },
})
