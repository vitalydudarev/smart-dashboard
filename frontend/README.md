# Frontend

React + TypeScript app built with Vite.

```bash
npm install
npm run dev
```

The look is defined by the design tokens in `src/styles/_tokens.scss`. The app follows the OS theme
unless the user picks one; the choice is stored in `localStorage` and written to `data-theme` on `<html>`.
