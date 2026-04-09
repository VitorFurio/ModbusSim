import { Link } from 'react-router-dom'
import { useSimStore } from '../store'
import type { Register, SignalKind } from '../types'

const SIGNAL_COLORS: Record<SignalKind, string> = {
  constant: 'border-slate-500',
  sine: 'border-blue-500',
  ramp: 'border-yellow-500',
  counter: 'border-green-500',
  counter_random: 'border-teal-500',
  random_walk: 'border-purple-500',
  step: 'border-orange-500',
}

const SIGNAL_BADGE: Record<SignalKind, string> = {
  constant: 'bg-slate-700 text-slate-300',
  sine: 'bg-blue-900/50 text-blue-300',
  ramp: 'bg-yellow-900/50 text-yellow-300',
  counter: 'bg-green-900/50 text-green-300',
  counter_random: 'bg-teal-900/50 text-teal-300',
  random_walk: 'bg-purple-900/50 text-purple-300',
  step: 'bg-orange-900/50 text-orange-300',
}

function Sparkline({ history }: { history: number[] }) {
  const values = history.filter((v) => v !== 0 || history.some((h) => h !== 0))
  if (values.length < 2) {
    return <div className="h-12 w-full bg-slate-800/50 rounded" />
  }

  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  const width = 200
  const height = 48
  const pts = values.map((v, i) => {
    const x = (i / (values.length - 1)) * width
    const y = height - ((v - min) / range) * height
    return `${x},${y}`
  })

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="w-full h-12"
      preserveAspectRatio="none"
    >
      <polyline
        points={pts.join(' ')}
        fill="none"
        stroke="#3b82f6"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function RegisterCard({ reg }: { reg: Register & { _history?: number[] } }) {
  const kind = reg.signal.kind
  const borderColor = SIGNAL_COLORS[kind] ?? 'border-slate-600'
  const badge = SIGNAL_BADGE[kind] ?? 'bg-slate-700 text-slate-300'
  const modbusAddr = reg.address + 40001

  const displayValue =
    reg.data_type === 'float32'
      ? reg.value.toFixed(2)
      : Math.round(reg.value).toString()

  return (
    <div
      className={`bg-slate-800 rounded-xl border-l-4 ${borderColor} p-4 flex flex-col gap-3`}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs text-slate-500 font-mono">#{modbusAddr}</p>
          <h3 className="text-white font-semibold text-sm mt-0.5">{reg.name}</h3>
          {reg.description && (
            <p className="text-xs text-slate-500 mt-0.5">{reg.description}</p>
          )}
        </div>
        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${badge}`}>
          {kind}
        </span>
      </div>

      <Sparkline history={reg._history ?? Array(30).fill(reg.value)} />

      <div className="flex items-baseline gap-1">
        <span className="text-2xl font-bold text-white font-mono">
          {displayValue}
        </span>
        {reg.unit && (
          <span className="text-slate-400 text-sm">{reg.unit}</span>
        )}
      </div>
    </div>
  )
}

export default function Dashboard() {
  const registers = useSimStore((s) => s.registers)

  if (registers.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center p-8">
        <p className="text-slate-500 text-lg mb-4">No registers configured</p>
        <Link
          to="/registers"
          className="text-blue-400 hover:text-blue-300 underline text-sm"
        >
          Go to Registers to add some
        </Link>
      </div>
    )
  }

  return (
    <div className="p-6">
      <h1 className="text-xl font-bold text-white mb-6">Dashboard</h1>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {registers.map((reg) => (
          <RegisterCard key={reg.id} reg={reg as Register & { _history?: number[] }} />
        ))}
      </div>
    </div>
  )
}
