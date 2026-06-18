import { fileURLToPath, URL } from 'node:url';

import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import ui from '@nuxt/ui/vite';
import vueDevTools from 'vite-plugin-vue-devtools';

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
    ui({
      ui: {
        colors: {
          primary: 'indigo',
          neutral: 'zinc',
        },
        textarea: {
          slots: {
            root: 'w-full',
          },
        },
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api/protocol': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/api/apps': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/api/admin': { target: 'http://127.0.0.1:8081', changeOrigin: true },
    },
  },
});
