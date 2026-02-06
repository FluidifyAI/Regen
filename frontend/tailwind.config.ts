import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      // OpenIncident Design Token System
      // Derived from logo: Steel blue shield (#2B6CB0), Amber siren (#E8842C)
      // Differentiates from incident.io's coral/orange brand

      colors: {
        // Brand (blue-primary from logo shield)
        brand: {
          primary: '#2563EB',
          'primary-hover': '#1D4ED8',
          'primary-light': '#DBEAFE',
        },

        // Accent (amber from logo siren - semantic use only)
        accent: {
          amber: '#F59E0B',
        },

        // Sidebar (dark navy theme)
        sidebar: {
          bg: '#0F172A',
          hover: '#1E293B',
          active: '#1E3A5F',
          text: '#94A3B8',
          'text-active': '#F1F5F9',
          border: '#1E293B',
        },

        // Surface (main content area)
        surface: {
          primary: '#FFFFFF',
          secondary: '#F8FAFC',
          tertiary: '#F1F5F9',
        },

        // Text
        text: {
          primary: '#0F172A',
          secondary: '#475569',
          tertiary: '#94A3B8',
        },

        // Borders
        border: {
          DEFAULT: '#E2E8F0',
          strong: '#CBD5E1',
        },

        // Semantic - Severity
        severity: {
          critical: '#DC2626',
          high: '#EA580C',
          medium: '#F59E0B',
          low: '#3B82F6',
        },

        // Semantic - Status
        status: {
          triggered: '#DC2626',
          acknowledged: '#F59E0B',
          resolved: '#16A34A',
        },
      },

      // Typography
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
      },

      fontSize: {
        xs: ['12px', { lineHeight: '16px' }],
        sm: ['13px', { lineHeight: '18px' }],
        base: ['14px', { lineHeight: '20px' }],
        lg: ['16px', { lineHeight: '24px' }],
        xl: ['18px', { lineHeight: '28px' }],
        '2xl': ['20px', { lineHeight: '28px' }],
        '3xl': ['24px', { lineHeight: '32px' }],
        'page-title': ['24px', { lineHeight: '32px', fontWeight: '600' }],
      },

      // Spacing tokens
      spacing: {
        'sidebar-width': '240px',
        'sidebar-collapsed': '56px',
        'content-padding': '24px',
        'section-gap': '16px',
        'properties-panel': '320px',
      },

      // Layout widths
      maxWidth: {
        'content': '1920px',
      },

      // Transitions
      transitionDuration: {
        '200': '200ms',
      },
    },
  },
  plugins: [],
} satisfies Config
