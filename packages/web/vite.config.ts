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
          // Never cache authenticated finance APIs in the SW Cache API
          // (holdings/NAV/portfolio must not linger on shared devices).
          runtimeCaching: [
            {
              urlPattern: /^\/api\/.*/i,
              handler: 'NetworkOnly',
            },
          ],
        },
      }),
      {
        name: 'preload-critical',
        transformIndexHtml(html) {
          // Vite auto-injects modulepreload for entry chunks.
          // Add preconnect for API origin to speed up first data fetch
          return html.replace('</head>', `
    <link rel="preconnect" href="/api" crossorigin>
    <link rel="dns-prefetch" href="/api">
  </head>`)
        },
      },
    ],
    server: {
      port: parseInt(env.VITE_PORT || '5176', 10),
      proxy: {
        '/api': apiTarget,
      },
    },
    build: {
      target: 'es2020',
      sourcemap: false,
      minify: 'esbuild',
      cssMinify: true,
      reportCompressedSize: false,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('node_modules')) {
              // Separate ECharts (508KB — rarely changes)
              if (id.includes('echarts')) return 'echarts'
              // Separate Kumo UI component library
              if (id.includes('@cloudflare/kumo')) return 'kumo'

              // Split the 549KB vendor blob into cache-friendly chunks
              // Phosphor icons — checked before react to avoid false match
              // (@phosphor-icons/react path contains /react/)
              if (id.includes('@phosphor-icons')) return 'icons'
              // React core — changes rarely, cache forever
              if (id.includes('react-dom') || id.includes('react/') || id.includes('scheduler')) return 'react'
              // React Router + its @remix-run/router dependency
              if (id.includes('react-router') || id.includes('@remix-run/router')) return 'router'
              // State + data fetching (zustand + all @tanstack/*)
              if (id.includes('zustand') || id.includes('@tanstack/')) return 'query'
              // i18n — only loaded when language switching is needed
              if (id.includes('i18next') || id.includes('react-i18next')) return 'i18n'
              // Zod + validation
              if (id.includes('zod')) return 'zod-vendor'

              return 'vendor' // small utils
            }
          },
        },
      },
    },
  }
})
