/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // "standalone" emits a self-contained server bundle (server.js + traced
  // node_modules) so the Docker runtime stage ships without the full
  // node_modules tree — required by the multi-stage Dockerfile.
  output: 'standalone',
  eslint: {
    dirs: ['src'],
  },
};

export default nextConfig;
