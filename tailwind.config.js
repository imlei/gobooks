// 遵循产品需求 v1.0
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./internal/web/templates/**/*.templ",
    "./internal/web/**/*.go"
  ],
  theme: {
    extend: {
      colors: {
        // Semantic color tokens (design system)
        primary: {
          DEFAULT: "#2563eb", // blue-600
          hover: "#1d4ed8", // blue-700
          soft: "#eff6ff", // blue-50
          focus: "#bfdbfe" // blue-200
        },
        success: {
          DEFAULT: "#16a34a",
          hover: "#15803d",
          soft: "#dcfce7",
          border: "#bbf7d0",
          focus: "#86efac"
        },
        warning: {
          DEFAULT: "#d97706", // amber-600
          hover: "#b45309", // amber-700
          soft: "#fffbeb", // amber-50
          border: "#fcd34d",
          focus: "#fbbf24"
        },
        danger: {
          DEFAULT: "#dc2626",
          hover: "#b91c1c",
          soft: "#fef2f2",
          border: "#fecaca",
          focus: "#fca5a5"
        },
        background: {
          DEFAULT: "#f9fafb" // gray-50
        },
        text: {
          DEFAULT: "#111827", // gray-900
          muted: "#6b7280", // gray-500
          muted2: "#4b5563", // gray-600
          muted3: "#374151" // gray-700
        },
        border: {
          DEFAULT: "#e5e7eb", // gray-200
          input: "#d1d5db", // gray-300
          subtle: "#f3f4f6", // gray-100
          danger: "#fecaca" // red-200
        },
        surface: {
          DEFAULT: "#ffffff" // white
        },
        onPrimary: "#ffffff",
        // Used for disabled states that previously used gray-200/gray-500 directly.
        disabled: {
          bg: "#e5e7eb", // gray-200
          text: "#6b7280" // gray-500
        },
        dangerText: {
          DEFAULT: "#dc2626" // red-600
        }
      },
      fontSize: {
        // Typography scale
        title: ["1.5rem", { lineHeight: "2rem" }], // 24px
        section: ["1rem", { lineHeight: "1.5rem" }], // 16px
        body: ["0.875rem", { lineHeight: "1.25rem" }], // 14px
        small: ["0.75rem", { lineHeight: "1rem" }], // 12px
      },
    }
  },
  plugins: []
};

