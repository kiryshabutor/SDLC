import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Разработка: API-сервер на :8080 (go run ./cmd/api), клиент на :5173.
// В рабочем окружении клиент и API отдаёт Caddy с одного адреса (DV-014).
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.js'],
  },
})
