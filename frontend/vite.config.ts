import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      // Forward API and SSE requests to the backend container
      '/api': 'http://localhost:8080',
      '/events': 'http://localhost:8080',
    },
  },
})