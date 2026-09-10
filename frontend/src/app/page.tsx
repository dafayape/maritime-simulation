import Link from 'next/link';

import { BackendStatus } from '@/components/home/BackendStatus';

const FEATURES = [
  {
    title: 'Topologi Mesh Live',
    body: 'Marker kapal dan garis rute multi-hop tergambar ulang seketika dari siaran WebSocket backend Go.',
  },
  {
    title: 'Injeksi Anomali',
    body: 'Picu badai buatan (packet loss) dan ganti Spreading Factor / TX power saat simulasi berjalan.',
  },
  {
    title: 'Telemetri Syahbandar',
    body: 'Setiap payload yang selamat melewati mesh terekam — lengkap dengan jalur hop yang dilaluinya.',
  },
];

export default function HomePage() {
  return (
    <main className="relative flex min-h-dvh flex-col items-center justify-center overflow-hidden px-4 py-16">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top,rgba(14,116,144,0.25),transparent_55%),radial-gradient(ellipse_at_bottom,rgba(30,64,175,0.2),transparent_55%)]" />

      <div className="relative z-10 flex w-full max-w-3xl flex-col items-center text-center">
        <p className="rounded-full border border-sky-500/30 bg-sky-500/10 px-3 py-1 text-[11px] font-medium uppercase tracking-widest text-sky-300">
          Maritime LoRa Mesh Network Simulator
        </p>
        <h1 className="mt-4 text-4xl font-bold leading-[1.15] tracking-tight sm:text-5xl">
          Dasbor Pemantauan
          {/* leading longgar + padding-bawah: text-5xl Tailwind memakai
              line-height 1 sehingga descender huruf 'g'/'J' pada teks gradien
              (bg-clip-text) terpotong; pb-1 memberi ruang turun huruf. */}
          <span className="block bg-gradient-to-r from-sky-300 to-emerald-300 bg-clip-text pb-1 leading-[1.2] text-transparent">
            Jaringan Mesh Maritim
          </span>
        </h1>
        <p className="mt-4 max-w-xl text-sm leading-relaxed text-slate-400">
          Visualisasi real-time armada kapal nelayan yang saling merelay data tangkapan lewat radio
          LoRa multi-hop hingga mendarat di Syahbandar — lengkap dengan kendali cuaca, Spreading
          Factor, dan skema payload dinamis.
        </p>

        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/setup"
            className="rounded-lg bg-sky-600 px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-sky-900/50 transition hover:bg-sky-500"
          >
            Mulai Simulasi
          </Link>
          <Link
            href="/reports"
            className="rounded-lg border border-white/10 bg-slate-900/70 px-6 py-3 text-sm font-semibold text-slate-200 transition hover:bg-slate-800"
          >
            Laporan Historis
          </Link>
        </div>

        <div className="mt-6">
          <BackendStatus />
        </div>

        <div className="mt-12 grid w-full gap-4 sm:grid-cols-3">
          {FEATURES.map((f) => (
            <div
              key={f.title}
              className="rounded-xl border border-white/10 bg-slate-900/60 p-4 text-left backdrop-blur"
            >
              <h2 className="text-sm font-semibold text-slate-100">{f.title}</h2>
              <p className="mt-1.5 text-xs leading-relaxed text-slate-400">{f.body}</p>
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}
