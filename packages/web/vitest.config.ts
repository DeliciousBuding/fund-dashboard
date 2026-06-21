import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    css: false,
    pool: 'forks',
    setupFiles: ['./src/__tests__/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
  resolve: {
    alias: [
      { find: '@cloudflare/kumo/components/chart', replacement: path.resolve(__dirname, './src/__tests__/mocks/kumo.tsx') },
      { find: '@cloudflare/kumo', replacement: path.resolve(__dirname, './src/__tests__/mocks/kumo.tsx') },
    ],
  },
})
