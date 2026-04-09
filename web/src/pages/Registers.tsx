import { useState } from 'react'
import { useSimStore } from '../store'
import { listRegisters, createRegister, updateRegister, deleteRegister } from '../api'
import type { Register, DataType, Signal, SignalKind } from '../types'
import { Plus, Pencil, Trash2, X } from 'lucide-react'

const DATA_TYPES: DataType[] = ['uint16', 'int16', 'uint32', 'int32', 'float32', 'bool']
const SIGNAL_KINDS: SignalKind[] = [
  'constant', 'sine', 'ramp', 'counter', 'counter_random', 'random_walk', 'step',
]

function emptySignal(kind: SignalKind): Signal {
  switch (kind) {
    case 'sine': return { kind, amplitude: 5, period: 30, offset: 0, min: 0, max: 0 }
    case 'ramp': return { kind, rate: 1, min: 0, max: 100 }
    case 'counter': return { kind, step_min: 1, interval_ms: 1000, min: 0, max: 100 }
    case 'counter_random': return { kind, step_min: 1, step_max: 5, interval_ms: 1000, min: 0, max: 100 }
    case 'random_walk': return { kind, step_max_walk: 1, min: 0, max: 100 }
    case 'step': return { kind, low: 0, high: 1, period: 5 }
    default: return { kind: 'constant', value: 0 }
  }
}

function emptyForm(): Omit<Register, 'value' | 'updated_at'> {
  return {
    id: '',
    name: '',
    description: '',
    address: 40001,
    data_type: 'float32',
    unit: '',
    signal: emptySignal('constant'),
  }
}

function SignalParams({
  signal,
  onChange,
}: {
  signal: Signal
  onChange: (s: Signal) => void
}) {
  const set = (key: keyof Signal, value: number | string) => {
    onChange({ ...signal, [key]: typeof value === 'string' ? parseFloat(value) || 0 : value })
  }

  const num = (label: string, key: keyof Signal, step = 'any') => (
    <div key={key}>
      <label className="block text-xs text-slate-400 mb-1">{label}</label>
      <input
        type="number"
        step={step}
        value={(signal[key] as number) ?? 0}
        onChange={(e) => set(key, e.target.value)}
        className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
      />
    </div>
  )

  switch (signal.kind) {
    case 'constant':
      return <>{num('Value', 'value')}</>
    case 'sine':
      return (
        <>
          {num('Amplitude', 'amplitude')}
          {num('Period (s)', 'period')}
          {num('Offset', 'offset')}
          {num('Min', 'min')}
          {num('Max', 'max')}
        </>
      )
    case 'ramp':
      return (
        <>
          {num('Rate (units/s)', 'rate')}
          {num('Min', 'min')}
          {num('Max', 'max')}
        </>
      )
    case 'counter':
      return (
        <>
          {num('Step', 'step_min')}
          {num('Interval (ms)', 'interval_ms', '1')}
          {num('Min', 'min')}
          {num('Max', 'max')}
        </>
      )
    case 'counter_random':
      return (
        <>
          {num('Step Min', 'step_min')}
          {num('Step Max', 'step_max')}
          {num('Interval (ms)', 'interval_ms', '1')}
          {num('Min', 'min')}
          {num('Max', 'max')}
        </>
      )
    case 'random_walk':
      return (
        <>
          {num('Max Step', 'step_max_walk')}
          {num('Min', 'min')}
          {num('Max', 'max')}
        </>
      )
    case 'step':
      return (
        <>
          {num('Low', 'low')}
          {num('High', 'high')}
          {num('Period (s)', 'period')}
        </>
      )
    default:
      return null
  }
}

interface ModalProps {
  initial: Omit<Register, 'value' | 'updated_at'>
  title: string
  onClose: () => void
  onSave: (r: Omit<Register, 'value' | 'updated_at'>) => Promise<void>
}

function RegisterModal({ initial, title, onClose, onSave }: ModalProps) {
  const [form, setForm] = useState({ ...initial })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const handleSignalKindChange = (kind: SignalKind) => {
    set('signal', emptySignal(kind))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    // Convert display address (40001+) to 0-based.
    const payload = { ...form, address: form.address - 40001 }
    try {
      await onSave(payload)
      onClose()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-slate-800 rounded-xl w-full max-w-lg max-h-[90vh] overflow-y-auto shadow-2xl">
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-700">
          <h2 className="font-semibold text-white">{title}</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white">
            <X size={18} />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          {error && (
            <p className="bg-red-900/40 border border-red-700 text-red-300 text-sm px-3 py-2 rounded">
              {error}
            </p>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-slate-400 mb-1">ID (optional)</label>
              <input
                value={form.id}
                onChange={(e) => set('id', e.target.value)}
                placeholder="auto"
                className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              />
            </div>
            <div>
              <label className="block text-xs text-slate-400 mb-1">Name *</label>
              <input
                required
                value={form.name}
                onChange={(e) => set('name', e.target.value)}
                className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs text-slate-400 mb-1">Description</label>
            <input
              value={form.description ?? ''}
              onChange={(e) => set('description', e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
            />
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-slate-400 mb-1">Modbus Address</label>
              <input
                type="number"
                min={40001}
                required
                value={form.address}
                onChange={(e) => set('address', parseInt(e.target.value) || 40001)}
                className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              />
            </div>
            <div>
              <label className="block text-xs text-slate-400 mb-1">Data Type</label>
              <select
                value={form.data_type}
                onChange={(e) => set('data_type', e.target.value as DataType)}
                className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              >
                {DATA_TYPES.map((dt) => (
                  <option key={dt} value={dt}>{dt}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-slate-400 mb-1">Unit</label>
              <input
                value={form.unit ?? ''}
                onChange={(e) => set('unit', e.target.value)}
                placeholder="°C, bar…"
                className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs text-slate-400 mb-1">Signal Kind</label>
            <select
              value={form.signal.kind}
              onChange={(e) => handleSignalKindChange(e.target.value as SignalKind)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
            >
              {SIGNAL_KINDS.map((k) => (
                <option key={k} value={k}>{k}</option>
              ))}
            </select>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <SignalParams
              signal={form.signal}
              onChange={(s) => set('signal', s)}
            />
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-slate-400 hover:text-white transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 text-white rounded-lg disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default function Registers() {
  const registers = useSimStore((s) => s.registers)
  const setRegisters = useSimStore((s) => s.setRegisters)
  const [modal, setModal] = useState<{
    mode: 'create' | 'edit'
    initial: Omit<Register, 'value' | 'updated_at'>
  } | null>(null)

  const refresh = () => listRegisters().then(setRegisters).catch(console.error)

  const handleCreate = async (r: Omit<Register, 'value' | 'updated_at'>) => {
    await createRegister(r)
    await refresh()
  }

  const handleUpdate = (id: string) => async (r: Omit<Register, 'value' | 'updated_at'>) => {
    await updateRegister(id, r)
    await refresh()
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this register?')) return
    await deleteRegister(id)
    await refresh()
  }

  const openCreate = () =>
    setModal({ mode: 'create', initial: emptyForm() })

  const openEdit = (reg: Register) => {
    setModal({
      mode: 'edit',
      initial: {
        ...reg,
        address: reg.address + 40001,
      },
    })
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-white">Registers</h1>
        <button
          onClick={openCreate}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium"
        >
          <Plus size={16} />
          Add Register
        </button>
      </div>

      {registers.length === 0 ? (
        <div className="text-center py-16 text-slate-500">
          <p>No registers. Add one to get started.</p>
        </div>
      ) : (
        <div className="bg-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase">
                <th className="text-left px-4 py-3">Address</th>
                <th className="text-left px-4 py-3">Name</th>
                <th className="text-left px-4 py-3">Type</th>
                <th className="text-left px-4 py-3">Signal</th>
                <th className="text-right px-4 py-3">Value</th>
                <th className="text-left px-4 py-3">Unit</th>
                <th className="text-right px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-700/50">
              {registers.map((reg) => (
                <tr key={reg.id} className="hover:bg-slate-700/30 transition-colors">
                  <td className="px-4 py-3 font-mono text-slate-400">
                    {reg.address + 40001}
                  </td>
                  <td className="px-4 py-3 text-white font-medium">{reg.name}</td>
                  <td className="px-4 py-3 text-slate-400 font-mono text-xs">{reg.data_type}</td>
                  <td className="px-4 py-3">
                    <span className="text-xs bg-slate-700 text-slate-300 px-2 py-0.5 rounded-full">
                      {reg.signal.kind}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-white">
                    {reg.data_type === 'float32'
                      ? reg.value.toFixed(2)
                      : Math.round(reg.value)}
                  </td>
                  <td className="px-4 py-3 text-slate-400">{reg.unit ?? '—'}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => openEdit(reg)}
                        className="text-slate-400 hover:text-blue-400 transition-colors"
                        title="Edit"
                      >
                        <Pencil size={15} />
                      </button>
                      <button
                        onClick={() => handleDelete(reg.id)}
                        className="text-slate-400 hover:text-red-400 transition-colors"
                        title="Delete"
                      >
                        <Trash2 size={15} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal && (
        <RegisterModal
          title={modal.mode === 'create' ? 'Add Register' : 'Edit Register'}
          initial={modal.initial}
          onClose={() => setModal(null)}
          onSave={
            modal.mode === 'create'
              ? handleCreate
              : handleUpdate(modal.initial.id)
          }
        />
      )}
    </div>
  )
}
