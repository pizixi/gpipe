import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

function getVendorChunkName(id: string) {
  if (!id.includes('node_modules')) {
    return undefined
  }

  if (id.includes('react-dom') || id.includes('react-router') || id.includes('react') || id.includes('scheduler')) {
    return 'vendor-react'
  }
  if (id.includes('react-i18next') || id.includes('i18next')) {
    return 'vendor-i18n'
  }
  if (id.includes('axios')) {
    return 'vendor-network'
  }
  return undefined
}

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          return getVendorChunkName(id)
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
