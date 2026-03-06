import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq, req) => {
            // Forward original host so the backend can build correct callback URIs
            // (e.g. Slack/SAML OAuth redirect_uri must match what the browser sees).
            proxyReq.setHeader('X-Forwarded-Host', req.headers.host ?? '')
          })
        },
      },
      // SAML SSO routes must hit the backend directly (not the Vite SPA).
      '/saml': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq, req) => {
            proxyReq.setHeader('X-Forwarded-Host', req.headers.host ?? '')
          })
        },
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
