import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'

export default createVuetify({
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        colors: {
          primary: '#1a73e8',
          secondary: '#5f6368',
          surface: '#ffffff',
          'surface-variant': '#f1f3f4',
          'on-surface': '#202124',
          'on-surface-variant': '#5f6368',
          error: '#d93025',
          success: '#1e8e3e',
          background: '#f8f9fa',
        },
      },
    },
  },
  defaults: {
    VBtn: {
      rounded: 'lg',
      fontWeight: '500',
    },
    VCard: {
      rounded: 'lg',
      elevation: 0,
      border: true,
    },
    VTextField: {
      variant: 'outlined',
      density: 'comfortable',
      hideDetails: 'auto',
    },
    VSelect: {
      variant: 'outlined',
      density: 'comfortable',
      hideDetails: 'auto',
    },
    VTextarea: {
      variant: 'outlined',
      density: 'comfortable',
      hideDetails: 'auto',
    },
    VSwitch: {
      color: 'primary',
      hideDetails: true,
      inset: true,
    },
  },
})
