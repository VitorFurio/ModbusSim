import { useState, useRef } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { listVersions, loadVersion, saveConfig, exportConfig, importConfig } from '../api'
import { listRegisters } from '../api'
import { useSimStore } from '../store'
import type { VersionInfo } from '../types'
import { Save, Upload, Download, RefreshCw } from 'lucide-react'

export default function Versions() {
  const qc = useQueryClient()
  const setRegisters = useSimStore((s) => s.setRegisters)
  const fileRef = useRef<HTMLInputElement>(null)

  const { data: versions = [], isLoading } = useQuery({
    queryKey: ['versions'],
    queryFn: listVersions,
    refetchInterval: 10000,
  })

  const [confirmLoad, setConfirmLoad] = useState<VersionInfo | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const handleSave = async () => {
    const name = prompt('Snapshot name (optional):') ?? ''
    setSaving(true)
    setError('')
    try {
      await saveConfig(name || undefined)
      qc.invalidateQueries({ queryKey: ['versions'] })
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const handleLoad = async (v: VersionInfo) => {
    try {
      await loadVersion(v.path)
      const regs = await listRegisters()
      setRegisters(regs)
      setConfirmLoad(null)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const text = await file.text()
    try {
      await importConfig(text)
      const regs = await listRegisters()
      setRegisters(regs)
      qc.invalidateQueries({ queryKey: ['versions'] })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    }
    e.target.value = ''
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-white">Config Versions</h1>
        <div className="flex items-center gap-3">
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium disabled:opacity-50"
          >
            <Save size={15} />
            Save Current
          </button>
          <button
            onClick={() => fileRef.current?.click()}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg text-sm font-medium"
          >
            <Upload size={15} />
            Import YAML
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".yaml,.yml"
            className="hidden"
            onChange={handleImport}
          />
          <button
            onClick={() => exportConfig().catch((e: unknown) => setError(String(e)))}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg text-sm font-medium"
          >
            <Download size={15} />
            Export
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 bg-red-900/40 border border-red-700 text-red-300 text-sm px-3 py-2 rounded">
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center py-16 text-slate-500">
          <RefreshCw className="animate-spin mr-2" size={16} />
          Loading…
        </div>
      ) : versions.length === 0 ? (
        <div className="text-center py-16 text-slate-500">
          No saved versions. Click "Save Current" to create one.
        </div>
      ) : (
        <div className="bg-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase">
                <th className="text-left px-4 py-3">Filename</th>
                <th className="text-left px-4 py-3">Name</th>
                <th className="text-left px-4 py-3">Saved At</th>
                <th className="text-center px-4 py-3">Registers</th>
                <th className="text-right px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-700/50">
              {versions.map((v) => (
                <tr key={v.path} className="hover:bg-slate-700/30 transition-colors">
                  <td className="px-4 py-3 font-mono text-xs text-slate-400">{v.filename}</td>
                  <td className="px-4 py-3 text-white">{v.name || '—'}</td>
                  <td className="px-4 py-3 text-slate-400">
                    {new Date(v.saved_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 text-center text-slate-400">{v.reg_count}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => setConfirmLoad(v)}
                        className="text-xs px-3 py-1 bg-blue-700 hover:bg-blue-600 text-white rounded transition-colors"
                      >
                        Load
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Confirm load dialog */}
      {confirmLoad && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-slate-800 rounded-xl p-6 max-w-sm w-full shadow-2xl">
            <h3 className="font-semibold text-white mb-3">Load version?</h3>
            <p className="text-sm text-slate-400 mb-5">
              This will replace all current registers with those from{' '}
              <span className="text-white font-mono">{confirmLoad.filename}</span>.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setConfirmLoad(null)}
                className="px-4 py-2 text-sm text-slate-400 hover:text-white"
              >
                Cancel
              </button>
              <button
                onClick={() => handleLoad(confirmLoad)}
                className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 text-white rounded-lg"
              >
                Load
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
