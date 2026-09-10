'use client';

import clsx from 'clsx';
import { useCallback, useEffect, useState } from 'react';

import { AnchorIcon, PencilIcon, PlusIcon, RefreshIcon, TrashIcon } from '@/components/ui/icons';
import { ApiError, createEdge, deleteEdge, listEdges, updateEdge } from '@/lib/api';
import type { Edge } from '@/types/backend';

type EditForm = { name: string; latitude: string; longitude: string };

/**
 * Master data Virtual Edge (Syahbandar). Migrasi backend sudah men-seed
 * EDGE-PRATU-01 (Pelabuhan Ratu); panel ini menampilkan daftarnya, form
 * tambah, dan per-baris Edit (nama/koordinat, kode tetap) + Hapus.
 */
export function EdgePanel() {
  const [edges, setEdges] = useState<Edge[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [form, setForm] = useState({ edge_code: '', name: '', latitude: '', longitude: '' });

  // Baris yang sedang disunting (null = tidak ada) + isi form-nya.
  const [editingCode, setEditingCode] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<EditForm>({ name: '', latitude: '', longitude: '' });
  const [savingEdit, setSavingEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  // Muat ulang daftar edge dari backend. `refreshing` memutar ikon selama
  // permintaan berlangsung sehingga tombolnya jelas benar-benar bekerja.
  const refresh = useCallback(() => {
    setRefreshing(true);
    setError(null);
    listEdges()
      .then((res) => setEdges(res ?? []))
      .catch(() => setError('backend tidak terjangkau'))
      .finally(() => setRefreshing(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleDelete = async (edge: Edge) => {
    const ok = window.confirm(
      `Hapus Virtual Edge "${edge.name}" (${edge.edge_code})?\n\n` +
        'Kapal yang memakainya sebagai gateway akan terisolasi hingga ada edge lain ' +
        'dalam jangkauan. Riwayat telemetri tetap tersimpan (ditandai tidak aktif).',
    );
    if (!ok) return;
    setDeleting(edge.edge_code);
    setError(null);
    try {
      await deleteEdge(edge.edge_code);
      refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal menghapus edge');
    } finally {
      setDeleting(null);
    }
  };

  const startEdit = (edge: Edge) => {
    setEditingCode(edge.edge_code);
    setEditForm({
      name: edge.name,
      latitude: String(edge.latitude),
      longitude: String(edge.longitude),
    });
    setEditError(null);
  };

  const cancelEdit = () => {
    setEditingCode(null);
    setEditError(null);
  };

  const saveEdit = async (code: string) => {
    setSavingEdit(true);
    setEditError(null);
    try {
      await updateEdge(code, {
        name: editForm.name.trim(),
        latitude: Number(editForm.latitude),
        longitude: Number(editForm.longitude),
      });
      setEditingCode(null);
      refresh();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : 'gagal menyimpan perubahan');
    } finally {
      setSavingEdit(false);
    }
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await createEdge({
        edge_code: form.edge_code.trim(),
        name: form.name.trim(),
        latitude: Number(form.latitude),
        longitude: Number(form.longitude),
      });
      setForm({ edge_code: '', name: '', latitude: '', longitude: '' });
      setShowForm(false);
      refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal menambah edge');
    } finally {
      setBusy(false);
    }
  };

  const inputCls =
    'w-full rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 text-xs text-slate-100 placeholder:text-slate-500';

  return (
    <div className="space-y-2">
      {edges === null && !error && <p className="text-xs text-slate-400">Memuat edge…</p>}
      {edges !== null && edges.length === 0 && (
        <p className="text-xs text-amber-300">
          Belum ada Virtual Edge — tanpa gateway Syahbandar, semua kapal akan terisolasi.
        </p>
      )}
      {edges?.map((edge) =>
        editingCode === edge.edge_code ? (
          <div
            key={edge.id}
            className="space-y-2 rounded-md border border-sky-500/30 bg-slate-800/70 p-3"
          >
            <p className="font-mono text-[10px] text-slate-400">
              {edge.edge_code} <span className="text-slate-500">(kode tidak dapat diubah)</span>
            </p>
            <div className="grid grid-cols-2 gap-2">
              <input
                placeholder="Nama pelabuhan"
                value={editForm.name}
                onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                required
                maxLength={150}
                className={clsx(inputCls, 'col-span-2')}
              />
              <input
                placeholder="Latitude (-6.98)"
                value={editForm.latitude}
                onChange={(e) => setEditForm({ ...editForm, latitude: e.target.value })}
                required
                inputMode="decimal"
                className={inputCls}
              />
              <input
                placeholder="Longitude (106.55)"
                value={editForm.longitude}
                onChange={(e) => setEditForm({ ...editForm, longitude: e.target.value })}
                required
                inputMode="decimal"
                className={inputCls}
              />
            </div>
            {editError && <p className="text-xs text-rose-300">{editError}</p>}
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => saveEdit(edge.edge_code)}
                disabled={savingEdit}
                className="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-40"
              >
                {savingEdit ? 'Menyimpan…' : 'Simpan Perubahan'}
              </button>
              <button
                type="button"
                onClick={cancelEdit}
                disabled={savingEdit}
                className="rounded-md px-3 py-1.5 text-xs text-slate-400 hover:bg-slate-800"
              >
                Batal
              </button>
            </div>
          </div>
        ) : (
          <div
            key={edge.id}
            className="flex items-center gap-2.5 rounded-md border border-white/5 bg-slate-800/50 px-3 py-2"
          >
            <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-cyan-500/15 text-cyan-300">
              <AnchorIcon size={15} />
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium text-slate-200" title={edge.name}>
                {edge.name}
              </p>
              <p className="truncate font-mono text-[10px] text-slate-400">
                {edge.edge_code} · {edge.latitude.toFixed(4)}, {edge.longitude.toFixed(4)}
              </p>
            </div>
            <button
              type="button"
              onClick={() => startEdit(edge)}
              aria-label={`Sunting edge ${edge.name}`}
              title="Sunting edge"
              className="inline-grid h-7 w-7 shrink-0 place-items-center rounded-md text-slate-400 transition hover:bg-sky-500/15 hover:text-sky-300 focus-visible:text-sky-300"
            >
              <PencilIcon size={14} />
            </button>
            <button
              type="button"
              onClick={() => handleDelete(edge)}
              disabled={deleting === edge.edge_code}
              aria-label={`Hapus edge ${edge.name}`}
              title="Hapus edge"
              className="inline-grid h-7 w-7 shrink-0 place-items-center rounded-md text-slate-400 transition hover:bg-rose-500/15 hover:text-rose-300 focus-visible:text-rose-300 disabled:opacity-40"
            >
              <TrashIcon size={14} className={clsx(deleting === edge.edge_code && 'animate-pulse')} />
            </button>
          </div>
        ),
      )}

      {showForm ? (
        <form onSubmit={submit} className="space-y-2 rounded-md border border-white/10 p-3">
          <div className="grid grid-cols-2 gap-2">
            <input
              placeholder="EDGE-KODE-01"
              value={form.edge_code}
              onChange={(e) => setForm({ ...form, edge_code: e.target.value })}
              required
              maxLength={50}
              className={inputCls}
            />
            <input
              placeholder="Nama pelabuhan"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
              maxLength={150}
              className={inputCls}
            />
            <input
              placeholder="Latitude (-6.98)"
              value={form.latitude}
              onChange={(e) => setForm({ ...form, latitude: e.target.value })}
              required
              inputMode="decimal"
              className={inputCls}
            />
            <input
              placeholder="Longitude (106.55)"
              value={form.longitude}
              onChange={(e) => setForm({ ...form, longitude: e.target.value })}
              required
              inputMode="decimal"
              className={inputCls}
            />
          </div>
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={busy}
              className="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-40"
            >
              {busy ? 'Menyimpan…' : 'Simpan Edge'}
            </button>
            <button
              type="button"
              onClick={() => setShowForm(false)}
              className="rounded-md px-3 py-1.5 text-xs text-slate-400 hover:bg-slate-800"
            >
              Batal
            </button>
          </div>
        </form>
      ) : (
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-slate-800 px-2.5 py-1.5 text-[11px] font-medium text-slate-200 transition hover:bg-slate-700"
          >
            <PlusIcon size={13} /> Tambah Virtual Edge
          </button>
          <button
            type="button"
            onClick={refresh}
            disabled={refreshing}
            aria-label="Muat ulang daftar edge"
            title="Muat ulang daftar edge"
            className="inline-grid h-7 w-7 place-items-center rounded-md text-slate-400 transition hover:bg-slate-800 hover:text-slate-200 disabled:opacity-60"
          >
            <RefreshIcon size={14} className={clsx(refreshing && 'animate-spin')} />
          </button>
        </div>
      )}
      {error && <p className="text-xs text-rose-300">{error}</p>}
    </div>
  );
}
