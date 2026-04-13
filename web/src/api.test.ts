import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  listDevices, createDevice, startDevice, stopDevice, deleteDevice,
  listRegisters, createRegister, deleteRegister,
  listVersions, saveVersion, loadVersion,
} from './api'

// Minimal fetch mock factory
function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(String(body)),
    blob: () => Promise.resolve(new Blob()),
  })
}

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('listDevices', () => {
  it('calls GET /api/devices', async () => {
    global.fetch = mockFetch(200, [])
    await listDevices()
    expect(fetch).toHaveBeenCalledWith('/api/devices', expect.objectContaining({ method: 'GET' }))
  })
})

describe('createDevice', () => {
  it('calls POST /api/devices with name/description/modbus_addr', async () => {
    const device = { id: 'd1', name: 'PLC', modbus_addr: ':5020', status: 'stopped', reg_count: 0 }
    global.fetch = mockFetch(200, device)
    const result = await createDevice({ name: 'PLC', description: 'test', modbus_addr: ':5020' })
    expect(fetch).toHaveBeenCalledWith('/api/devices', expect.objectContaining({ method: 'POST' }))
    expect(result).toEqual(device)
  })
})

describe('startDevice / stopDevice', () => {
  it('calls POST /api/devices/{id}/start', async () => {
    global.fetch = mockFetch(200, { id: 'd1', status: 'running' })
    await startDevice('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/start', expect.objectContaining({ method: 'POST' }))
  })

  it('calls POST /api/devices/{id}/stop', async () => {
    global.fetch = mockFetch(200, { id: 'd1', status: 'stopped' })
    await stopDevice('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/stop', expect.objectContaining({ method: 'POST' }))
  })
})

describe('deleteDevice', () => {
  it('calls DELETE /api/devices/{id} and handles 204', async () => {
    global.fetch = mockFetch(204, undefined)
    await deleteDevice('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('listRegisters', () => {
  it('calls GET /api/devices/{id}/registers', async () => {
    global.fetch = mockFetch(200, [])
    await listRegisters('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/registers', expect.objectContaining({ method: 'GET' }))
  })
})

describe('createRegister', () => {
  it('calls POST /api/devices/{id}/registers', async () => {
    const r = { id: '', name: 'Temp', address: 100, data_type: 'float32' as const, signal: { kind: 'constant' as const, value: 0 } }
    global.fetch = mockFetch(200, { ...r, value: 0, updated_at: 0 })
    await createRegister('d1', r)
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/registers', expect.objectContaining({ method: 'POST' }))
  })
})

describe('deleteRegister', () => {
  it('calls DELETE /api/devices/{id}/registers/{rid}', async () => {
    global.fetch = mockFetch(204, undefined)
    await deleteRegister('d1', 'r1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/registers/r1', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('listVersions', () => {
  it('calls GET /api/devices/{id}/versions', async () => {
    global.fetch = mockFetch(200, [])
    await listVersions('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/versions', expect.objectContaining({ method: 'GET' }))
  })
})

describe('saveVersion', () => {
  it('calls POST /api/devices/{id}/versions/save', async () => {
    global.fetch = mockFetch(200, { path: 'configs/d1/v1.yaml' })
    await saveVersion('d1')
    expect(fetch).toHaveBeenCalledWith('/api/devices/d1/versions/save', expect.objectContaining({ method: 'POST' }))
  })
})

describe('loadVersion', () => {
  it('calls POST /api/devices/{id}/versions/load with path in body', async () => {
    global.fetch = mockFetch(200, { status: 'ok' })
    await loadVersion('d1', 'configs/d1/v1.yaml')
    const call = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(call[0]).toBe('/api/devices/d1/versions/load')
    expect(JSON.parse(call[1].body)).toEqual({ path: 'configs/d1/v1.yaml' })
  })
})

describe('error handling', () => {
  it('throws when response is not ok', async () => {
    global.fetch = mockFetch(500, 'internal error')
    await expect(listDevices()).rejects.toThrow('500')
  })
})
