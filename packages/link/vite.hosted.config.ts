import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  root: ".",
  base: "/static/link/",
  build: {
    outDir: "dist-hosted",
    emptyOutDir: true,
  },
});
