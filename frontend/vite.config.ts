import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { resolve } from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        logs: resolve(__dirname, 'logs.html'),
        detections: resolve(__dirname, 'detections.html'),
        camera: resolve(__dirname, 'camera.html'),
        whitelist: resolve(__dirname, 'whitelist.html'),
      },
    },
  },
  server: {
    proxy: {
      // Forward API and SSE requests to the backend container
      '/api': 'http://localhost:8080',
      '/events': 'http://localhost:8080',
      // Forward camera/vision requests to the vision server
      '/cam': 'http://localhost:8090',
    },
  },
})