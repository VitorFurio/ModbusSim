import { useState, useRef } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { listVersions, loadVersion, saveVersion, exportConfig, importConfig, listRegisters } from '../api'
import { useSimStore } from '../store'
import type { VersionInfo } from '../types'
import { Save, Upload, Download, RefreshCw, AlertCircle } from 'lucide-react'

export default function Versions() {
  const qc = useQueryClient()
  const setRegisters = useSimStore((s) => s.setRegisters)
  const selectedDeviceId = useSimStore((s) => s.selectedDeviceId)
  const fileRef = useRef<HTMLInputElement>(null)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['versions', selectedDeviceId],
    queryFn: () => (selectedDeviceId ? listVersions(selectedDeviceId) : Promise.resolve([])),
    enabled: !!selectedDeviceId,
    refetchInterval: 15000,
    refetchOnWindowFocus: false,
    retry: 1,
  })

  const versions: VersionInfo[] = Array.isArray(data) ? data : []

  const [confirmLoad, setConfirmLoad] = useState<VersionInfo | null>(null)
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(false)
  const [pageError, setPageError] = useState('')

  const handleSave = async () => {
    if (!selectedDeviceId) return
    setSaving(true)
    setPageError('')
    try {
      await saveVersion(selectedDeviceId)
      await qc.invalidateQueries({ queryKey: ['versions', selectedDeviceId] })
    } catch (e: unknown) {
      setPageError(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const handleLoad = async (v: VersionInfo) => {
    if (!selectedDeviceId) return
    setLoading(true)
    setPageError('')
    try {
      await loadVersion(selectedDeviceId, v.path)
      const regs = await listRegisters(selectedDeviceId)
      setRegisters(regs)
      setConfirmLoad(null)
      await qc.invalidateQueries({ queryKey: ['registers', selectedDeviceId] })
    } catch (e: unknown) {
      setPageError(e instanceof Error ? e.message : String(e))
      setConfirmLoad(null)
    } finally {
      setLoading(false)
    }
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!selectedDeviceId) return
    const file = e.target.files?.[0]
    if (!file) return
    setPageError('')
    const text = await file.text()
    try {
      await importConfig(selectedDeviceId, text)
      const regs = await listRegisters(selectedDeviceId)
      setRegisters(regs)
      await qc.invalidateQueries({ queryKey: ['versions', selectedDeviceId] })
    } catch (err: unknown) {
      setPageError(err instanceof Error ? err.message : String(err))
    }
    e.target.value = ''
  }

  if (!selectedDeviceId) {
    return (
      <div className="flex items-center justify-center h-full text-slate-500">
        Select a device to manage versions.
      </div>
    )
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-white">Config Versions</h1>
        <div className="flex items-center gap-3">
          <button
            onClick={handleSave}
            disabled={saving || loading}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium disabled:opacity-50"
          >
            {saving ? <RefreshCw size={15} className="animate-spin" /> : <Save size={15} />}
            {saving ? 'Saving…' : 'Save Current'}
          </button>
          <button
            onClick={() => fileRef.current?.click()}
            disabled={saving || loading}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg text-sm font-medium disabled:opacity-50"
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
            onClick={() =>
              exportConfig(selectedDeviceId).catch((e: unknown) => setPageError(String(e)))
            }
            disabled={saving || loading}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg text-sm font-medium disabled:opacity-50"
          >
            <Download size={15} />
            Export
          </button>
        </div>
      </div>

      {pageError && (
        <div className="mb-4 flex items-start gap-2 bg-red-900/40 border border-red-700 text-red-300 text-sm px-3 py-2 rounded">
          <AlertCircle size={15} className="mt-0.5 shrink-0" />
          <span>{pageError}</span>
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center py-16 text-slate-500">
          <RefreshCw className="animate-spin mr-2" size={16} />
          Loading…
        </div>
      ) : isError ? (
        <div className="flex flex-col items-center justify-center py-16 gap-3 text-slate-500">
          <AlertCircle size={32} className="text-slate-600" />
          <p className="text-sm">
            {error instanceof Error ? error.message : 'Failed to load versions'}
          </p>
          <button
            onClick={() => refetch()}
            className="text-xs px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded"
          >
            Retry
          </button>
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
                        disabled={loading}
                        className="text-xs px-3 py-1 bg-blue-700 hover:bg-blue-600 text-white rounded transition-colors disabled:opacity-50"
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
                disabled={loading}
                className="px-4 py-2 text-sm text-slate-400 hover:text-white disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={() => handleLoad(confirmLoad)}
                disabled={loading}
                className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 text-white rounded-lg disabled:opacity-50"
              >
                {loading && <RefreshCw size={13} className="animate-spin" />}
                {loading ? 'Loading…' : 'Load'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
