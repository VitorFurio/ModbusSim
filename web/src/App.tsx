import { BrowserRouter, Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import type { LucideProps } from 'lucide-react'
import { LayoutDashboard, List, Archive, Activity, Plus, Play, Square, Trash2, Server, X } from 'lucide-react'
import { useWebSocket } from './ws'
import { useSimStore } from './store'
import { listDevices, listRegisters, createDevice, startDevice, stopDevice, deleteDevice } from './api'
import type { DeviceInfo } from './types'
import Dashboard from './pages/Dashboard'
import Registers from './pages/Registers'
import Versions from './pages/Versions'

const queryClient = new QueryClient()

type LucideIcon = React.ForwardRefExoticComponent<Omit<LucideProps, 'ref'> & React.RefAttributes<SVGSVGElement>>

function NavItem({ to, icon: Icon, label }: { to: string; icon: LucideIcon; label: string }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
          isActive
            ? 'bg-blue-600 text-white'
            : 'text-slate-400 hover:bg-slate-700 hover:text-white'
        }`
      }
    >
      <Icon size={18} />
      {label}
    </NavLink>
  )
}

// ─── Add Device Modal ─────────────────────────────────────────────────────────

interface AddDeviceModalProps {
  onClose: () => void
  onCreated: (dev: DeviceInfo) => void
}

function AddDeviceModal({ onClose, onCreated }: AddDeviceModalProps) {
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [addr, setAddr] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      const dev = await createDevice({ name, description: desc || undefined, modbus_addr: addr || undefined })
      onCreated(dev)
      onClose()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-slate-800 rounded-xl w-full max-w-sm shadow-2xl">
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-700">
          <h2 className="font-semibold text-white text-sm">Add Device</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white">
            <X size={16} />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-5 space-y-3">
          {error && (
            <p className="bg-red-900/40 border border-red-700 text-red-300 text-xs px-3 py-2 rounded">
              {error}
            </p>
          )}
          <div>
            <label className="block text-xs text-slate-400 mb-1">Name *</label>
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
              placeholder="PLC 300"
            />
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">Description</label>
            <input
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white"
            />
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">
              Modbus Address <span className="text-slate-500">(auto if empty)</span>
            </label>
            <input
              value={addr}
              onChange={(e) => setAddr(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 rounded px-2 py-1.5 text-sm text-white font-mono"
              placeholder=":5021"
            />
          </div>
          <div className="flex justify-end gap-3 pt-1">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-slate-400 hover:text-white"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 text-white rounded-lg disabled:opacity-50"
            >
              {saving ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Device List Item ─────────────────────────────────────────────────────────

interface DeviceItemProps {
  device: DeviceInfo
  isSelected: boolean
  onSelect: () => void
  onToggle: () => void
  onDelete: () => void
}

function DeviceItem({ device, isSelected, onSelect, onToggle, onDelete }: DeviceItemProps) {
  const isRunning = device.status === 'running'

  return (
    <div
      className={`group flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer transition-colors ${
        isSelected ? 'bg-slate-700' : 'hover:bg-slate-700/50'
      }`}
      onClick={onSelect}
    >
      <span
        className={`w-2 h-2 rounded-full flex-shrink-0 ${
          isRunning ? 'bg-green-400' : 'bg-slate-500'
        }`}
      />
      <div className="flex-1 min-w-0">
        <p className="text-xs font-medium text-white truncate">{device.name}</p>
        <p className="text-xs text-slate-500 font-mono">{device.modbus_addr}</p>
      </div>
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          onClick={(e) => { e.stopPropagation(); onToggle() }}
          title={isRunning ? 'Stop' : 'Start'}
          className="p-1 rounded text-slate-400 hover:text-white"
        >
          {isRunning ? <Square size={12} /> : <Play size={12} />}
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete() }}
          title="Delete"
          className="p-1 rounded text-slate-400 hover:text-red-400"
        >
          <Trash2 size={12} />
        </button>
      </div>
    </div>
  )
}

// ─── AppShell ─────────────────────────────────────────────────────────────────

function AppShell() {
  const devices = useSimStore((s) => s.devices)
  const selectedDeviceId = useSimStore((s) => s.selectedDeviceId)
  const setDevices = useSimStore((s) => s.setDevices)
  const updateDevice = useSimStore((s) => s.updateDevice)
  const removeDevice = useSimStore((s) => s.removeDevice)
  const selectDevice = useSimStore((s) => s.selectDevice)
  const setRegisters = useSimStore((s) => s.setRegisters)

  const { connected } = useWebSocket(selectedDeviceId)
  const location = useLocation()
  const [showAddModal, setShowAddModal] = useState(false)

  // Load device list on mount.
  useEffect(() => {
    listDevices()
      .then(setDevices)
      .catch(console.error)
  }, [setDevices])

  // Load registers when selected device or page changes.
  useEffect(() => {
    if (!selectedDeviceId) return
    listRegisters(selectedDeviceId)
      .then(setRegisters)
      .catch(console.error)
  }, [selectedDeviceId, location.pathname, setRegisters])

  const handleToggle = async (dev: DeviceInfo) => {
    try {
      const updated =
        dev.status === 'running'
          ? await stopDevice(dev.id)
          : await startDevice(dev.id)
      updateDevice(updated)
    } catch (err) {
      console.error('toggle device:', err)
    }
  }

  const handleDelete = async (dev: DeviceInfo) => {
    if (!confirm(`Delete device "${dev.name}"?`)) return
    try {
      await deleteDevice(dev.id)
      removeDevice(dev.id)
    } catch (err) {
      console.error('delete device:', err)
    }
  }

  const handleCreated = (dev: DeviceInfo) => {
    setDevices([...devices, dev])
    selectDevice(dev.id)
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className="w-56 flex-shrink-0 bg-slate-900 border-r border-slate-700/50 flex flex-col">
        {/* Logo */}
        <div className="px-4 py-5 border-b border-slate-700/50">
          <div className="flex items-center gap-2">
            <Activity size={20} className="text-blue-400" />
            <span className="font-semibold text-white text-sm">ModbusSim</span>
          </div>
        </div>

        {/* Device list */}
        <div className="border-b border-slate-700/50 flex-shrink-0">
          <div className="flex items-center justify-between px-4 py-2">
            <div className="flex items-center gap-1.5 text-xs text-slate-500 uppercase tracking-wider">
              <Server size={11} />
              Devices
            </div>
            <button
              onClick={() => setShowAddModal(true)}
              className="text-slate-500 hover:text-slate-300 transition-colors"
              title="Add Device"
            >
              <Plus size={14} />
            </button>
          </div>
          <div className="px-2 pb-2 space-y-0.5 max-h-48 overflow-y-auto">
            {devices.length === 0 ? (
              <p className="text-xs text-slate-600 px-2 py-1">No devices</p>
            ) : (
              devices.map((dev) => (
                <DeviceItem
                  key={dev.id}
                  device={dev}
                  isSelected={dev.id === selectedDeviceId}
                  onSelect={() => selectDevice(dev.id)}
                  onToggle={() => handleToggle(dev)}
                  onDelete={() => handleDelete(dev)}
                />
              ))
            )}
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 p-3 space-y-1">
          <NavItem to="/" icon={LayoutDashboard} label="Dashboard" />
          <NavItem to="/registers" icon={List} label="Registers" />
          <NavItem to="/versions" icon={Archive} label="Versions" />
        </nav>

        {/* Connection status */}
        <div className="px-4 py-3 border-t border-slate-700/50">
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <span
              className={`w-2 h-2 rounded-full ${
                connected ? 'bg-green-500' : 'bg-red-500'
              }`}
            />
            {connected ? 'Connected' : 'Disconnected'}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto bg-slate-950">
        {selectedDeviceId === null ? (
          <div className="flex flex-col items-center justify-center h-full text-slate-500">
            <Server size={40} className="mb-3 text-slate-700" />
            <p>Select or create a device to get started</p>
          </div>
        ) : (
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/registers" element={<Registers />} />
            <Route path="/versions" element={<Versions />} />
          </Routes>
        )}
      </main>

      {showAddModal && (
        <AddDeviceModal
          onClose={() => setShowAddModal(false)}
          onCreated={handleCreated}
        />
      )}
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppShell />
      </BrowserRouter>
    </QueryClientProvider>
  )
}
