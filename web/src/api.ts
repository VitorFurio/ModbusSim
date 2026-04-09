import type { Register, AppConfig, VersionInfo } from './types'

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

// Registers
export const listRegisters = () => req<Register[]>('GET', '/api/registers')
export const createRegister = (r: Omit<Register, 'value' | 'updated_at'>) =>
  req<Register>('POST', '/api/registers', r)
export const updateRegister = (id: string, r: Omit<Register, 'value' | 'updated_at'>) =>
  req<Register>('PUT', `/api/registers/${id}`, r)
export const deleteRegister = (id: string) => req<void>('DELETE', `/api/registers/${id}`)

// Config
export const getConfig = () => req<AppConfig>('GET', '/api/config')
export const saveConfig = (name?: string, description?: string) =>
  req<{ path: string }>('POST', '/api/config/save', { name, description })

// Versions
export const listVersions = () => req<VersionInfo[]>('GET', '/api/versions')
export const loadVersion = (path: string) =>
  req<{ status: string }>('POST', '/api/versions/load', { path })

export const exportConfig = async (): Promise<void> => {
  const res = await fetch('/api/versions/export')
  if (!res.ok) throw new Error('export failed')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'modbussim.yaml'
  a.click()
  URL.revokeObjectURL(url)
}

export const importConfig = (yaml: string) =>
  req<{ status: string }>('POST', '/api/versions/import', undefined, yaml, {
    'Content-Type': 'application/yaml',
  })
