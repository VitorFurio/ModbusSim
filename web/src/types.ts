export type DataType = 'uint16' | 'int16' | 'uint32' | 'int32' | 'float32' | 'bool'

export type SignalKind =
  | 'constant'
  | 'sine'
  | 'ramp'
  | 'counter'
  | 'counter_random'
  | 'random_walk'
  | 'step'

export interface Signal {
  kind: SignalKind
  value?: number
  amplitude?: number
  period?: number
  offset?: number
  rate?: number
  step_min?: number
  step_max?: number
  interval_ms?: number
  step_max_walk?: number
  low?: number
  high?: number
  min?: number
  max?: number
}

export interface Register {
  id: string
  name: string
  description?: string
  address: number
  data_type: DataType
  unit?: string
  signal: Signal
  value: number
  updated_at: number
}

export interface RegisterSnapshot {
  id: string
  value: number
  updated_at: number
  history: number[]
}

export interface WSMessage {
  type: 'snapshot'
  registers: RegisterSnapshot[]
}

export interface AppConfig {
  name: string
  description?: string
  modbus_addr: string
  admin_addr: string
}

export interface VersionInfo {
  path: string
  filename: string
  saved_at: string
  name: string
  reg_count: number
}
