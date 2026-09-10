import clsx from 'clsx';

/**
 * Kartu panel tembus pandang (glassmorphism, SRS §6.2) yang dipakai seluruh
 * overlay dasbor: latar gelap semi-transparan + backdrop-blur Tailwind.
 */
export function GlassCard({
  title,
  action,
  className,
  children,
}: {
  title?: string;
  action?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section
      className={clsx(
        'rounded-xl border border-white/10 bg-slate-900/70 shadow-lg shadow-black/30 backdrop-blur-md',
        className,
      )}
    >
      {(title || action) && (
        <header className="flex items-center justify-between px-4 pb-1 pt-3">
          {title && (
            <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              {title}
            </h2>
          )}
          {action}
        </header>
      )}
      <div className={clsx('px-4 pb-4', title || action ? 'pt-1' : 'pt-4')}>{children}</div>
    </section>
  );
}
