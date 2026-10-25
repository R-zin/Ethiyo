/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

const proxyConfig = {
  '/api': {
    target: 'http://localhost:8080',
    changeOrigin: true,
  },
  '/health': {
    target: 'http://localhost:8080',
    changeOrigin: true,
  },
}

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
  server: {
    port: 3000,
    proxy: proxyConfig,
  },
  preview: {
    port: 3000,
    proxy: proxyConfig,
  },
})
