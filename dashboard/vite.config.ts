import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    allowedHosts: true,
    proxy: {
      '/api': {
        target: 'http://localhost:911',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:911',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:1314',
        ws: true,
        changeOrigin: true,
      },
    },
  },
  css: {
    preprocessorOptions: {
      less: {
        javascriptEnabled: true,
        modifyVars: {
          '@brand-color': '#3b82f6',
          '@text-color-primary': '#f9fafb',
          '@text-color-secondary': '#9ca3af',
          '@bg-color-page': '#030712',
          '@bg-color-container': '#111827',
          '@bg-color-container-hover': '#1f2937',
          '@border-color': '#374151',
        },
      },
    },
  },
})
