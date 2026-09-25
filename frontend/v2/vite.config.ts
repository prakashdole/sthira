import { defineConfig } from 'vite';

const backendTarget = process.env.VITE_BACKEND_URL || process.env.STHIRA_BACKEND_URL || 'http://127.0.0.1:8080';
const uiPort = process.env.VITE_PORT ? parseInt(process.env.VITE_PORT, 10) : 5173;

export default defineConfig({
  optimizeDeps: {
    exclude: ['maplibre-gl'],
  },
  server: {
    host: '127.0.0.1',
    port: uiPort,
    allowedHosts: ['king.tail9d0b65.ts.net', '.localhost.run', '.lhr.life'],
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
      },
      '/health': {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
});
