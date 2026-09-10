/**
 * Set ikon SVG inline (satu keluarga visual, stroke 1.75, currentColor).
 *
 * Dibuat sendiri alih-alih menarik dependensi (lucide/heroicons) agar image
 * Docker tetap ramping & hermetis di VPS tanpa akses keluar. Semua ikon:
 *  - memakai viewBox 24 dan mewarisi warna teks lewat `stroke="currentColor"`,
 *  - `aria-hidden` secara default (ikon dekoratif) — tombol IKON-SAJA yang
 *    memakainya WAJIB memberi `aria-label` pada elemen <button>-nya,
 *  - ukuran seragam lewat prop `size` (token: 14 / 16 / 20) demi ritme visual.
 */

interface IconProps {
  size?: number;
  className?: string;
}

function Svg({
  size = 16,
  className,
  children,
}: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
      focusable="false"
    >
      {children}
    </svg>
  );
}

/** Muat ulang / segarkan data. */
export function RefreshIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
      <path d="M21 3v5h-5" />
      <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
      <path d="M3 21v-5h5" />
    </Svg>
  );
}

/** Tambah item. */
export function PlusIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </Svg>
  );
}

/** Hapus / buang. */
export function TrashIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M3 6h18" />
      <path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
      <path d="M10 11v6" />
      <path d="M14 11v6" />
    </Svg>
  );
}

/** Jangkar — penanda Virtual Edge (Syahbandar). */
export function AnchorIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <circle cx="12" cy="5" r="2" />
      <path d="M12 22V7" />
      <path d="M5 12H2a10 10 0 0 0 20 0h-3" />
    </Svg>
  );
}

/** Isi otomatis / contoh preset. */
export function SparklesIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M12 3l1.9 4.6L18.5 9.5 13.9 11.4 12 16l-1.9-4.6L5.5 9.5l4.6-1.9L12 3z" />
      <path d="M19 15l.7 1.8L21.5 17.5l-1.8.7L19 20l-.7-1.8L16.5 17.5l1.8-.7L19 15z" />
    </Svg>
  );
}

/** Sunting / edit. */
export function PencilIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z" />
      <path d="M15 5l4 4" />
    </Svg>
  );
}

/** Hentikan (aksi destruktif). */
export function StopIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <rect x="6" y="6" width="12" height="12" rx="2" />
    </Svg>
  );
}

/** Laporan / dokumen telemetri. */
export function FileTextIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5z" />
      <path d="M14 3v5h5" />
      <path d="M9 13h6" />
      <path d="M9 17h6" />
    </Svg>
  );
}

export function ChevronLeftIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M15 18l-6-6 6-6" />
    </Svg>
  );
}

export function ChevronRightIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M9 18l6-6-6-6" />
    </Svg>
  );
}

/** Panah kembali (navigasi ke halaman sebelumnya). */
export function ArrowLeftIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M19 12H5" />
      <path d="M12 19l-7-7 7-7" />
    </Svg>
  );
}
