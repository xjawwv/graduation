export default defineNuxtConfig({
  compatibilityDate: '2025-03-01',
  devtools: { enabled: false },
  modules: ['@nuxtjs/tailwindcss'],
  css: ['@fontsource-variable/ibm-plex-sans', '~/assets/css/main.css'],
  app: {
    head: {
      title: 'Graduation Live',
      htmlAttrs: { lang: 'en' },
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
        { name: 'theme-color', content: '#0d1014' },
      ],
    },
  },
  runtimeConfig: {
    public: {
      apiBase: 'http://localhost:8080',
      wsUrl: 'ws://localhost:8080/ws',
    },
  },
  typescript: { strict: true, typeCheck: true },
})
