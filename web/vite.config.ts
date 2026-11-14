import { defineConfig, configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
    restoreMocks: true,
    // A stray duplicated dependency copy ("node_modules 5.56.17 AM") sits next
    // to the real one; its bundled spec files are not this project's tests.
    exclude: [...configDefaults.exclude, '**/node_modules */**'],
  },
});
