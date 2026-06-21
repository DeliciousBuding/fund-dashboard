import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_API_TARGET || 'http://127.0.0.1:8765'

  return {
    plugins: [
      react(),
      VitePWA({
        registerType: 'autoUpdate',
        includeAssets: ['ndaq.svg', 'ndaq-d.svg', 'ndaq-big.svg', 'ndaq-big-d.svg'],
        manifest: {
          name: 'TokenDance Fund',
          short_name: 'TokenDance',
          description: 'TokenDance Fund — 综合投资仪表盘',
          start_url: '/',
          display: 'standalone',
          orientation: 'any',
          background_color: '#0a0e14',
          theme_color: '#0090ba',
          icons: [
            {
              src: '/ndaq.svg',
              sizes: 'any',
              type: 'image/svg+xml',
              purpose: 'any maskable',
            },
            {
              src: '/ndaq-d.svg',
              sizes: 'any',
              type: 'image/svg+xml',
              purpose: 'any maskable',
            },
          ],
          categories: ['finance', 'productivity'],
          lang: 'zh-CN',
        },
        workbox: {
          globPatterns: ['**/*.{js,css,html,svg,png,ico,woff2}'],
          runtimeCaching: [
            {
              urlPattern: /^\/api\/.*/i,
              handler: 'NetworkFirst',
              options: {
                cacheName: 'api-cache',
                expiration: {
                  maxEntries: 200,
                  maxAgeSeconds: 60 * 60, // 1 hour
                },
                networkTimeoutSeconds: 10,
                cacheableResponse: {
                  statuses: [0, 200],
                },
              },
            },
          ],
        },
      }),
    ],
    server: {
      port: parseInt(env.VITE_PORT || '5176', 10),
      proxy: {
        '/api': apiTarget,
      },
    },
    build: {
      target: 'es2020',
      sourcemap: mode === 'production' ? false : 'inline',
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('node_modules')) {
              if (id.includes('echarts')) return 'echarts'
              if (id.includes('@cloudflare/kumo')) return 'kumo'
              return 'vendor'
            }
          },
        },
      },
    },
  }
})
