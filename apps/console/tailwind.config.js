/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: {
          0: "#0A0A0A",
          50: "#101010",
          100: "#141414",
          200: "#1A1A1A",
          300: "#1E1E1E",
          400: "#252525",
          500: "#2E2E2E",
          600: "#3A3A3A",
          700: "#555555",
          800: "#888888",
          900: "#B8B8B8",
          950: "#E8E8E8",
        },
        phosphor: {
          DEFAULT: "#7FE787",
          dim: "#4FAB57",
          glow: "#A8FFB0",
        },
        amber: {
          DEFAULT: "#FFB84D",
          dim: "#B07A1F",
        },
        danger: {
          DEFAULT: "#FF6B6B",
          dim: "#A03F3F",
        },
        info: {
          DEFAULT: "#5DA8FF",
          dim: "#2E6BAA",
        },
      },
      fontFamily: {
        sans: ["'IBM Plex Sans'", "system-ui", "sans-serif"],
        display: ["'IBM Plex Sans Condensed'", "'IBM Plex Sans'", "sans-serif"],
        mono: ["'IBM Plex Mono'", "ui-monospace", "monospace"],
      },
      fontSize: {
        "2xs": ["0.6875rem", { lineHeight: "1rem", letterSpacing: "0.06em" }],
        xs: ["0.75rem", { lineHeight: "1.125rem" }],
      },
      letterSpacing: {
        wider: "0.08em",
        widest: "0.18em",
      },
      borderRadius: {
        none: "0",
        sm: "2px",
        DEFAULT: "3px",
        md: "4px",
      },
      backgroundImage: {
        scanlines:
          "repeating-linear-gradient(0deg, rgba(255,255,255,0.012) 0px, rgba(255,255,255,0.012) 1px, transparent 1px, transparent 3px)",
        noise:
          "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='3'/%3E%3CfeColorMatrix values='0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 0.04 0'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E\")",
      },
      animation: {
        "pulse-dot": "pulseDot 1.8s ease-in-out infinite",
        "fade-up": "fadeUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) both",
      },
      keyframes: {
        pulseDot: {
          "0%, 100%": { opacity: "1", transform: "scale(1)" },
          "50%": { opacity: "0.4", transform: "scale(0.85)" },
        },
        fadeUp: {
          "0%": { opacity: "0", transform: "translateY(8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
    },
  },
  plugins: [],
};
