import type { Metadata } from 'next';
import Link from 'next/link';

import { EdgePanel } from '@/components/setup/EdgePanel';
import { SessionForm } from '@/components/setup/SessionForm';
import { SessionList } from '@/components/setup/SessionList';
import { GlassCard } from '@/components/ui/GlassCard';
import { ArrowLeftIcon } from '@/components/ui/icons';

export const metadata: Metadata = {
  title: 'Setup Simulasi',
};

/**
 * Halaman konfigurasi pra-simulasi (SRS §6.1): kartu terpusat berisi form
 * sesi + schema builder, master data edge, dan daftar sesi tersimpan.
 */
export default function SetupPage() {
  return (
    <main className="min-h-dvh bg-[radial-gradient(ellipse_at_top,rgba(14,116,144,0.18),transparent_60%)] px-4 py-10">
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-5">
        <header className="text-center">
          <Link
            href="/"
            className="inline-flex items-center gap-1 text-xs text-slate-400 transition hover:text-slate-200"
          >
            <ArrowLeftIcon size={13} /> Beranda
          </Link>
          <h1 className="mt-2 text-2xl font-bold tracking-tight">Setup Simulasi</h1>
          <p className="mt-1 text-sm text-slate-400">
            Konfigurasikan parameter radio LoRa dan struktur payload, lalu buka dasbor pemantauan.
          </p>
        </header>

        <GlassCard title="Sesi Baru">
          <SessionForm />
        </GlassCard>

        <GlassCard title="Virtual Edge (Gateway Syahbandar)">
          <EdgePanel />
        </GlassCard>

        <GlassCard title="Sesi Tersimpan">
          <SessionList />
        </GlassCard>
      </div>
    </main>
  );
}
