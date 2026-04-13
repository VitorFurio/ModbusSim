import type { Register, DeviceInfo, VersionInfo } from './types'

async function req<T>(
  method: string,
  url: string,
  body?: unknown,
  rawBody?: BodyInit,
  extraHeaders?: Record<string, string>
): Promise<T> {
  const headers: Record<string, string> = { ...extraHeaders }
  let bodyInit: BodyInit | undefined
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    bodyInit = JSON.stringify(body)
  } else if (rawBody !== undefined) {
    bodyInit = rawBody
  }
  const res = await fetch(url, { method, headers, body: bodyInit })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`${res.status} ${res.statusText}: ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

const d = (id: string) => `/api/devices/${id}`

// ── Devices ───────────────────────────────────────────────────────────────────

export const listDevices = () =>
  req<DeviceInfo[]>('GET', '/api/devices')

export const createDevice = (body: { name: string; description?: string; modbus_addr?: string }) =>
  req<DeviceInfo>('POST', '/api/devices', body)

export const getDevice = (id: string) =>
  req<DeviceInfo>('GET', d(id))

export const updateDevice = (id: string, body: { name?: string; description?: string; modbus_addr?: string }) =>
  req<DeviceInfo>('PUT', d(id), body)

export const deleteDevice = (id: string) =>
  req<void>('DELETE', d(id))

export const startDevice = (id: string) =>
  req<DeviceInfo>('POST', `${d(id)}/start`)

export const stopDevice = (id: string) =>
  req<DeviceInfo>('POST', `${d(id)}/stop`)

// ── Registers (device-scoped) ─────────────────────────────────────────────────

export const listRegisters = (deviceId: string) =>
  req<Register[]>('GET', `${d(deviceId)}/registers`)

export const createRegister = (deviceId: string, r: Omit<Register, 'value' | 'updated_at'>) =>
  req<Register>('POST', `${d(deviceId)}/registers`, r)

export const updateRegister = (deviceId: string, id: string, r: Omit<Register, 'value' | 'updated_at'>) =>
  req<Register>('PUT', `${d(deviceId)}/registers/${id}`, r)

export const deleteRegister = (deviceId: string, id: string) =>
  req<void>('DELETE', `${d(deviceId)}/registers/${id}`)

// ── Versions (device-scoped) ──────────────────────────────────────────────────

export const saveVersion = (deviceId: string) =>
  req<{ path: string }>('POST', `${d(deviceId)}/versions/save`)

export const listVersions = (deviceId: string) =>
  req<VersionInfo[]>('GET', `${d(deviceId)}/versions`)

export const loadVersion = (deviceId: string, path: string) =>
  req<{ status: string }>('POST', `${d(deviceId)}/versions/load`, { path })

export const exportConfig = async (deviceId: string): Promise<void> => {
  const res = await fetch(`${d(deviceId)}/versions/export`)
  if (!res.ok) throw new Error('export failed')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${deviceId}.yaml`
  a.click()
  URL.revokeObjectURL(url)
}

export const importConfig = (deviceId: string, yaml: string) =>
  req<{ status: string }>('POST', `${d(deviceId)}/versions/import`, undefined, yaml, {
    'Content-Type': 'application/yaml',
  })
