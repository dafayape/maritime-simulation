'use client';

import { PlusIcon, SparklesIcon, TrashIcon } from '@/components/ui/icons';
import { estimatePackedBytes, FIELD_TYPE_OPTIONS, isValidFieldName, MAX_SCHEMA_FIELDS } from '@/lib/lora';
import type { SchemaField } from '@/types/backend';

const PRESET_FIELDS: SchemaField[] = [
  { name: 'jenis_ikan', type: 'string_16' },
  { name: 'berat_kg', type: 'float32' },
  { name: 'suhu_c', type: 'float32' },
];

/**
 * Pembangun skema payload dinamis (PRD "Dynamic Schema Form Builder").
 * Estimasi byte di bawah form adalah mirror klien dari
 * domain.EstimatePackedBytes — angka final yang mengikat tetap dihitung
 * backend saat POST /schema.
 */
export function SchemaBuilder({
  fields,
  onChange,
  payloadLimit,
}: {
  fields: SchemaField[];
  onChange: (fields: SchemaField[]) => void;
  payloadLimit: number;
}) {
  const estimate = estimatePackedBytes(fields);
  const overLimit = fields.length > 0 && estimate > payloadLimit;

  const update = (index: number, patch: Partial<SchemaField>) => {
    onChange(fields.map((f, i) => (i === index ? { ...f, ...patch } : f)));
  };

  return (
    <fieldset className="space-y-2">
      <div className="flex items-center justify-between">
        <legend className="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Skema Payload Dinamis
        </legend>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => onChange(PRESET_FIELDS)}
            title="Isi dengan skema contoh"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-200 transition hover:bg-slate-700"
          >
            <SparklesIcon size={13} /> Isi contoh
          </button>
          <button
            type="button"
            disabled={fields.length >= MAX_SCHEMA_FIELDS}
            onClick={() => onChange([...fields, { name: '', type: 'float32' }])}
            title="Tambah field baru"
            className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-200 transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <PlusIcon size={13} /> Tambah field
          </button>
        </div>
      </div>

      {fields.length === 0 && (
        <p className="rounded-md border border-dashed border-white/10 px-3 py-2 text-xs text-slate-400">
          Opsional — tanpa skema, kapal tidak tahu struktur data tangkapan yang harus dikirim.
        </p>
      )}

      {fields.map((field, index) => {
        const nameInvalid = field.name !== '' && !isValidFieldName(field.name);
        return (
          <div key={index} className="flex items-start gap-2">
            <div className="flex-1">
              <input
                value={field.name}
                onChange={(e) => update(index, { name: e.target.value })}
                placeholder={`nama_field_${index + 1}`}
                className="w-full rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 font-mono text-xs text-slate-100 placeholder:text-slate-500"
              />
              {nameInvalid && (
                <p className="mt-0.5 text-[10px] text-rose-300">
                  Huruf/angka/underscore, diawali huruf, maks 30 karakter.
                </p>
              )}
            </div>
            <select
              value={field.type}
              onChange={(e) => update(index, { type: e.target.value })}
              className="rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 font-mono text-xs text-slate-100"
            >
              {FIELD_TYPE_OPTIONS.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
            <button
              type="button"
              onClick={() => onChange(fields.filter((_, i) => i !== index))}
              className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-rose-300 transition hover:bg-rose-500/15 hover:text-rose-200"
              aria-label={`Hapus field ${field.name || index + 1}`}
              title="Hapus field"
            >
              <TrashIcon size={15} />
            </button>
          </div>
        );
      })}

      {fields.length > 0 && (
        <p className={`text-[11px] ${overLimit ? 'text-rose-300' : 'text-slate-400'}`}>
          Estimasi payload terpaket: ±{estimate} byte / batas SF saat ini {payloadLimit} byte
          {overLimit && ' — paket AKAN DI-DROP oleh simulator. Kecilkan skema atau turunkan SF.'}
        </p>
      )}
    </fieldset>
  );
}
