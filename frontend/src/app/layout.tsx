import type { Metadata, Viewport } from 'next';

import './globals.css';

export const metadata: Metadata = {
  title: {
    default: 'Maritime LoRa Mesh Simulator',
    template: '%s · Maritime LoRa Mesh Simulator',
  },
  description:
    'Dasbor pemantauan real-time jaringan LoRa mesh maritim: topologi kapal, rute multi-hop, dan statistik transmisi.',
};

export const viewport: Viewport = {
  themeColor: '#0f172a',
};

// Font memakai system stack (Tailwind font-sans) alih-alih next/font/google:
// build image Docker harus tetap hermetis di VPS tanpa akses keluar, dan
// dasbor internal tidak butuh brand font ber-lisensi.
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body className="font-sans">{children}</body>
    </html>
  );
}
