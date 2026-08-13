/// <reference types="vitest/config" />
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';

const OUT_DIR = '../internal/web/dist';

/* `emptyOutDir` wipes the .gitkeep that lets internal/web compile before the
   client has ever been built, so put it back after every build. */
function keepOutDir(): Plugin {
  let dir = '';
  return {
    name: 'snagarr-keep-out-dir',
    apply: 'build',
    configResolved(config) {
      dir = resolve(config.root, config.build.outDir);
    },
    closeBundle() {
      mkdirSync(dir, { recursive: true });
      writeFileSync(resolve(dir, '.gitkeep'), '');
    },
  };
}

export default defineConfig({
  base: './',
  plugins: [react(), keepOutDir()],
  build: {
    outDir: OUT_DIR,
    emptyOutDir: true,
    reportCompressedSize: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
