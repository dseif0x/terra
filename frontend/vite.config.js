import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { chunkSizeWarningLimit: 1500 },
  worker: { format: 'es' },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
