import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      external: ['fsevents'],
    },
  },
  server: {
    proxy: {
      '/game.v1': {
        target: 'http://localhost:4838',
        changeOrigin: true,
      },
      '/oauth': {
        target: 'http://localhost:4838',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Host', 'localhost:4838');
          });
        },
      },
      '/login': {
        target: 'http://localhost:4838',
        changeOrigin: true,
      },
    },
  },
})
