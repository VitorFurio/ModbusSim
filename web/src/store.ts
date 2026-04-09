import { create } from 'zustand'
import type { Register, RegisterSnapshot } from './types'

interface SimState {
  registers: Register[]
  connected: boolean
  setRegisters: (regs: Register[]) => void
  updateValues: (snaps: RegisterSnapshot[]) => void
  setConnected: (v: boolean) => void
}

export const useSimStore = create<SimState>((set) => ({
  registers: [],
  connected: false,

  setRegisters: (regs) => set({ registers: regs }),

  updateValues: (snaps) =>
    set((state) => {
      const snapMap = new Map(snaps.map((s) => [s.id, s]))
      return {
        registers: state.registers.map((r) => {
          const snap = snapMap.get(r.id)
          if (!snap) return r
          return {
            ...r,
            value: snap.value,
            updated_at: snap.updated_at,
            _history: snap.history,
          } as Register & { _history: number[] }
        }),
      }
    }),

  setConnected: (v) => set({ connected: v }),
}))

// Extend Register with optional _history for rendering.
declare module './types' {
  interface Register {
    _history?: number[]
  }
}
