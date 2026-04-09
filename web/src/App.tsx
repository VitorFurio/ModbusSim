import { BrowserRouter, Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEffect } from 'react'
import type { LucideProps } from 'lucide-react'
import { LayoutDashboard, List, Archive, Activity } from 'lucide-react'
import { useWebSocket } from './ws'
import { useSimStore } from './store'
import { listRegisters } from './api'
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

function AppShell() {
  const { connected } = useWebSocket()
  const setRegisters = useSimStore((s) => s.setRegisters)
  const location = useLocation()

  // Load registers on mount and on navigation.
  useEffect(() => {
    listRegisters()
      .then(setRegisters)
      .catch(console.error)
  }, [location.pathname, setRegisters])

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className="w-56 flex-shrink-0 bg-slate-900 border-r border-slate-700/50 flex flex-col">
        <div className="px-4 py-5 border-b border-slate-700/50">
          <div className="flex items-center gap-2">
            <Activity size={20} className="text-blue-400" />
            <span className="font-semibold text-white text-sm">ModbusSim</span>
          </div>
        </div>

        <nav className="flex-1 p-3 space-y-1">
          <NavItem to="/" icon={LayoutDashboard} label="Dashboard" />
          <NavItem to="/registers" icon={List} label="Registers" />
          <NavItem to="/versions" icon={Archive} label="Versions" />
        </nav>

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
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/registers" element={<Registers />} />
          <Route path="/versions" element={<Versions />} />
        </Routes>
      </main>
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
