import { describe, it, expect, beforeEach } from 'vitest'
import { useSimStore } from './store'
import type { DeviceInfo, Register } from './types'

const dev1: DeviceInfo = { id: 'd1', name: 'Dev1', modbus_addr: ':5020', status: 'stopped', reg_count: 0 }
const dev2: DeviceInfo = { id: 'd2', name: 'Dev2', modbus_addr: ':5021', status: 'running', reg_count: 3 }

const reg: Register = {
  id: 'r1', name: 'Temp', address: 100, data_type: 'float32',
  signal: { kind: 'constant', value: 25 }, value: 25, updated_at: 0,
}

function getStore() {
  return useSimStore.getState()
}

beforeEach(() => {
  useSimStore.setState({
    devices: [],
    selectedDeviceId: null,
    registers: [],
    connected: false,
  })
})

describe('setDevices', () => {
  it('stores the device list', () => {
    getStore().setDevices([dev1, dev2])
    expect(getStore().devices).toEqual([dev1, dev2])
  })

  it('auto-selects first device when nothing is selected', () => {
    getStore().setDevices([dev1, dev2])
    expect(getStore().selectedDeviceId).toBe('d1')
  })

  it('does not change selection when a device is already selected', () => {
    useSimStore.setState({ selectedDeviceId: 'd2' })
    getStore().setDevices([dev1, dev2])
    expect(getStore().selectedDeviceId).toBe('d2')
  })

  it('handles empty list without crashing', () => {
    getStore().setDevices([])
    expect(getStore().devices).toEqual([])
    expect(getStore().selectedDeviceId).toBeNull()
  })
})

describe('updateDevice', () => {
  it('replaces a device by id', () => {
    useSimStore.setState({ devices: [dev1, dev2] })
    const updated: DeviceInfo = { ...dev1, status: 'running' }
    getStore().updateDevice(updated)
    expect(getStore().devices.find((d) => d.id === 'd1')?.status).toBe('running')
  })
})

describe('removeDevice', () => {
  it('removes the device and keeps selection if another device exists', () => {
    useSimStore.setState({ devices: [dev1, dev2], selectedDeviceId: 'd2' })
    getStore().removeDevice('d2')
    expect(getStore().devices).toEqual([dev1])
    expect(getStore().selectedDeviceId).toBe('d1')
  })

  it('clears selection when last device is removed', () => {
    useSimStore.setState({ devices: [dev1], selectedDeviceId: 'd1' })
    getStore().removeDevice('d1')
    expect(getStore().selectedDeviceId).toBeNull()
  })

  it('clears registers when the selected device is removed', () => {
    useSimStore.setState({ devices: [dev1], selectedDeviceId: 'd1', registers: [reg] })
    getStore().removeDevice('d1')
    expect(getStore().registers).toEqual([])
  })

  it('keeps registers when a non-selected device is removed', () => {
    useSimStore.setState({ devices: [dev1, dev2], selectedDeviceId: 'd1', registers: [reg] })
    getStore().removeDevice('d2')
    expect(getStore().registers).toEqual([reg])
  })
})

describe('selectDevice', () => {
  it('sets selectedDeviceId and clears registers', () => {
    useSimStore.setState({ registers: [reg] })
    getStore().selectDevice('d1')
    expect(getStore().selectedDeviceId).toBe('d1')
    expect(getStore().registers).toEqual([])
  })
})

describe('setRegisters', () => {
  it('stores registers', () => {
    getStore().setRegisters([reg])
    expect(getStore().registers).toEqual([reg])
  })
})

describe('updateValues', () => {
  it('updates value and updated_at for matching register ids', () => {
    useSimStore.setState({ registers: [reg] })
    getStore().updateValues([{ id: 'r1', value: 99, updated_at: 123, history: [1, 2, 3] }])
    const updated = getStore().registers[0]
    expect(updated.value).toBe(99)
    expect(updated.updated_at).toBe(123)
    expect((updated as typeof reg & { _history?: number[] })._history).toEqual([1, 2, 3])
  })

  it('leaves registers unchanged when id does not match', () => {
    useSimStore.setState({ registers: [reg] })
    getStore().updateValues([{ id: 'unknown', value: 99, updated_at: 123, history: [] }])
    expect(getStore().registers[0].value).toBe(25)
  })
})

describe('setConnected', () => {
  it('toggles connected flag', () => {
    getStore().setConnected(true)
    expect(getStore().connected).toBe(true)
    getStore().setConnected(false)
    expect(getStore().connected).toBe(false)
  })
})
