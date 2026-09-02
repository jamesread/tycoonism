import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const backendPort = process.env.PORT || '4838'
const backendTarget = `http://localhost:${backendPort}`

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
        target: backendTarget,
        changeOrigin: true,
      },
      '/oauth': {
        target: backendTarget,
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Host', `localhost:${backendPort}`);
          });
        },
      },
      '/login': {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
})
