import { defineConfig } from 'vite'
import { sveltekit } from '@sveltejs/kit/vite'

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    proxy: {
      '/api': {
        target: 'https://localhost:1323',
        changeOrigin: true,
        secure: false
      },
      '/bootstrap': {
        target: 'https://localhost:1323',
        changeOrigin: true,
        secure: false
      }
    }
  }
})
