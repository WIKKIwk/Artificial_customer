/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./index.html",
    "./{App,components,utils}/**/*.{ts,tsx,js,jsx}",
    "./*.{ts,tsx,js,jsx}"
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};
