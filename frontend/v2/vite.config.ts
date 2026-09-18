import { defineConfig } from 'vite';

export default defineConfig({
  optimizeDeps: {
    exclude: ['maplibre-gl'],
  },
  server: {
    host: '127.0.0.1',
    allowedHosts: ['king.tail9d0b65.ts.net', '.localhost.run', '.lhr.life'],
    proxy: {
      '/api': 'http://127.0.0.1:8000',
    },
  },
});
