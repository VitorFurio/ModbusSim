import { create } from 'zustand'
import type { Register, RegisterSnapshot, DeviceInfo } from './types'

interface SimState {
  // Device list and selection
  devices: DeviceInfo[]
  selectedDeviceId: string | null
  setDevices: (devices: DeviceInfo[]) => void
  updateDevice: (info: DeviceInfo) => void
  removeDevice: (id: string) => void
  selectDevice: (id: string) => void

  // Per-device register state (scoped to selectedDeviceId)
  registers: Register[]
  connected: boolean
  setRegisters: (regs: Register[]) => void
  updateValues: (snaps: RegisterSnapshot[]) => void
  setConnected: (v: boolean) => void
}

export const useSimStore = create<SimState>((set) => ({
  devices: [],
  selectedDeviceId: null,
  registers: [],
  connected: false,

  setDevices: (devices) =>
    set((state) => ({
      devices: devices ?? [],
      // Auto-select first device if nothing selected yet
      selectedDeviceId:
        state.selectedDeviceId === null && devices.length > 0
          ? devices[0].id
          : state.selectedDeviceId,
    })),

  updateDevice: (info) =>
    set((state) => ({
      devices: state.devices.map((d) => (d.id === info.id ? info : d)),
    })),

  removeDevice: (id) =>
    set((state) => {
      const remaining = state.devices.filter((d) => d.id !== id)
      return {
        devices: remaining,
        selectedDeviceId:
          state.selectedDeviceId === id
            ? remaining.length > 0
              ? remaining[0].id
              : null
            : state.selectedDeviceId,
        registers: state.selectedDeviceId === id ? [] : state.registers,
      }
    }),

  selectDevice: (id) =>
    set({ selectedDeviceId: id, registers: [] }),

  setRegisters: (regs) => set({ registers: regs ?? [] }),

  updateValues: (snaps) =>
    set((state) => {
      if (!snaps || !state.registers) return {}
      const snapMap = new Map(snaps.map((s) => [s.id, s]))
      return {
        registers: state.registers.map((r) => {
          const snap = snapMap.get(r.id)
          if (!snap) return r
          return {
            ...r,
            value: snap.value,
            updated_at: snap.updated_at,
            _history: snap.history ?? [],
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
