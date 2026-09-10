import type { Config } from 'tailwindcss';

const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Node status palette locked by the SRS (§3A): connected / retrying /
        // isolated. Referenced from map markers and the node list panel.
        node: {
          connected: '#10B981',
          retrying: '#F59E0B',
          isolated: '#EF4444',
        },
      },
    },
  },
  plugins: [],
};

export default config;
