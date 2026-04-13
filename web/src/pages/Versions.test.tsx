import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useSimStore } from '../store'
import type { VersionInfo } from '../types'
import Versions from './Versions'

vi.mock('../api', () => ({
  listVersions: vi.fn(),
  saveVersion: vi.fn(),
  loadVersion: vi.fn(),
  listRegisters: vi.fn(),
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
}))

import * as api from '../api'

const mockVersion: VersionInfo = {
  path: 'configs/d1/v1.yaml',
  filename: 'v1.yaml',
  saved_at: '2024-01-15T10:30:00Z',
  name: 'v1',
  reg_count: 5,
}

function renderWithQuery(ui: React.ReactElement) {
  // retry:0 for defaultOptions but component overrides with retry:1 — see loadVersion test notes
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

beforeEach(() => {
  vi.clearAllMocks()
  useSimStore.setState({
    devices: [{ id: 'd1', name: 'PLC', modbus_addr: ':5020', status: 'running', reg_count: 5 }],
    selectedDeviceId: 'd1',
    registers: [],
    connected: true,
  })
  vi.mocked(api.listVersions).mockResolvedValue([mockVersion])
  vi.mocked(api.listRegisters).mockResolvedValue([])
})

describe('Versions page — no device selected', () => {
  it('shows prompt when no device is selected', () => {
    useSimStore.setState({ selectedDeviceId: null })
    renderWithQuery(<Versions />)
    expect(screen.getByText(/select a device/i)).toBeInTheDocument()
  })
})

describe('Versions page — with device', () => {
  it('renders the version list after loading', async () => {
    renderWithQuery(<Versions />)
    await waitFor(() => {
      expect(screen.getByText('v1.yaml')).toBeInTheDocument()
    })
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('shows empty state when no versions exist', async () => {
    vi.mocked(api.listVersions).mockResolvedValue([])
    renderWithQuery(<Versions />)
    await waitFor(() => {
      expect(screen.getByText(/no saved versions/i)).toBeInTheDocument()
    })
  })

  it('calls saveVersion when "Save Current" is clicked', async () => {
    const user = userEvent.setup()
    vi.mocked(api.saveVersion).mockResolvedValue({ path: 'configs/d1/v2.yaml' })

    renderWithQuery(<Versions />)
    await waitFor(() => screen.getByText('v1.yaml'))

    await user.click(screen.getByRole('button', { name: /save current/i }))

    await waitFor(() => {
      expect(api.saveVersion).toHaveBeenCalledWith('d1')
    })
  })

  it('shows error banner when saveVersion fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.saveVersion).mockRejectedValue(new Error('disk full'))

    renderWithQuery(<Versions />)
    await waitFor(() => screen.getByText('v1.yaml'))

    await user.click(screen.getByRole('button', { name: /save current/i }))

    await waitFor(() => {
      expect(screen.getByText('disk full')).toBeInTheDocument()
    })
  })

  it('opens confirmation modal when "Load" is clicked', async () => {
    const user = userEvent.setup()
    renderWithQuery(<Versions />)
    await waitFor(() => screen.getByText('v1.yaml'))

    await user.click(screen.getByRole('button', { name: /^load$/i }))

    expect(screen.getByText(/load version\?/i)).toBeInTheDocument()
    // Filename appears inside the modal confirmation text
    expect(screen.getByText('v1.yaml', { selector: 'span' })).toBeInTheDocument()
  })

  it('calls loadVersion and closes modal when confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(api.loadVersion).mockResolvedValue({ status: 'ok' })

    renderWithQuery(<Versions />)
    await waitFor(() => screen.getByText('v1.yaml'))

    // Click "Load" in the version table row
    await user.click(screen.getByRole('button', { name: /^load$/i }))

    // Now a confirmation modal appears — click the "Load" button inside the modal
    const modal = screen.getByText(/load version\?/i).closest('div')!
    await user.click(within(modal).getByRole('button', { name: /^load$/i }))

    await waitFor(() => {
      expect(api.loadVersion).toHaveBeenCalledWith('d1', 'configs/d1/v1.yaml')
    })
    // Modal closes after success
    await waitFor(() => {
      expect(screen.queryByText(/load version\?/i)).not.toBeInTheDocument()
    })
  })

  it('closes confirmation modal on cancel', async () => {
    const user = userEvent.setup()
    renderWithQuery(<Versions />)
    await waitFor(() => screen.getByText('v1.yaml'))

    await user.click(screen.getByRole('button', { name: /^load$/i }))
    expect(screen.getByText(/load version\?/i)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /cancel/i }))
    expect(screen.queryByText(/load version\?/i)).not.toBeInTheDocument()
  })

  it('shows error state when listVersions fails', async () => {
    vi.mocked(api.listVersions).mockRejectedValue(new Error('network error'))
    renderWithQuery(<Versions />)
    // Component retries once (retry:1) before showing error — allow extra time
    await waitFor(() => {
      expect(screen.getByText(/network error|failed to load/i)).toBeInTheDocument()
    }, { timeout: 5000 })
  })
})
