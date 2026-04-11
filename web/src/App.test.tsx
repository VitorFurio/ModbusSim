import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSimStore } from './store'
import type { DeviceInfo } from './types'
import App from './App'

// Mock API calls
vi.mock('./api', () => ({
  listDevices: vi.fn(),
  createDevice: vi.fn(),
  startDevice: vi.fn(),
  stopDevice: vi.fn(),
  deleteDevice: vi.fn(),
  listRegisters: vi.fn(),
  listVersions: vi.fn(),
}))

// Mock WebSocket hook so tests don't open real WS connections
vi.mock('./ws', () => ({
  useWebSocket: () => ({ connected: false }),
}))

import * as api from './api'

const dev1: DeviceInfo = { id: 'd1', name: 'PLC300', modbus_addr: ':5020', status: 'stopped', reg_count: 2 }
const dev2: DeviceInfo = { id: 'd2', name: 'Inverter', modbus_addr: ':5021', status: 'running', reg_count: 0 }

function renderApp() {
  useSimStore.setState({ devices: [], selectedDeviceId: null, registers: [], connected: false })
  // App already provides its own QueryClientProvider — render it directly
  return render(<App />)
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.listDevices).mockResolvedValue([dev1])
  vi.mocked(api.listRegisters).mockResolvedValue([])
  vi.mocked(api.listVersions).mockResolvedValue([])
})

describe('AppShell — device list', () => {
  it('loads and displays devices on mount', async () => {
    renderApp()
    await waitFor(() => {
      expect(screen.getByText('PLC300')).toBeInTheDocument()
    })
    expect(screen.getByText(':5020')).toBeInTheDocument()
  })

  it('shows multiple devices in the sidebar', async () => {
    vi.mocked(api.listDevices).mockResolvedValue([dev1, dev2])
    renderApp()
    await waitFor(() => screen.getByText('Inverter'))
    expect(screen.getByText('PLC300')).toBeInTheDocument()
    expect(screen.getByText(':5021')).toBeInTheDocument()
  })

  it('auto-selects first device on load', async () => {
    renderApp()
    await waitFor(() => {
      expect(useSimStore.getState().selectedDeviceId).toBe('d1')
    })
  })

  it('shows "no devices" text when device list is empty', async () => {
    vi.mocked(api.listDevices).mockResolvedValue([])
    renderApp()
    await waitFor(() => {
      expect(screen.getByText(/no devices/i)).toBeInTheDocument()
    })
  })
})

describe('AppShell — Add Device modal', () => {
  it('opens the Add Device modal when + button is clicked', async () => {
    const user = userEvent.setup()
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    await user.click(screen.getByTitle('Add Device'))
    expect(screen.getByText('Add Device', { selector: 'h2' })).toBeInTheDocument()
  })

  it('creates a device and adds it to the list', async () => {
    const user = userEvent.setup()
    const newDev: DeviceInfo = { id: 'd2', name: 'Inverter', modbus_addr: ':5021', status: 'stopped', reg_count: 0 }
    vi.mocked(api.createDevice).mockResolvedValue(newDev)

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    await user.click(screen.getByTitle('Add Device'))
    // Use placeholder to find the name input (labels lack htmlFor)
    await user.type(screen.getByPlaceholderText('PLC 300'), 'Inverter')
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(api.createDevice).toHaveBeenCalledWith(expect.objectContaining({ name: 'Inverter' }))
    })
    expect(screen.queryByText('Add Device', { selector: 'h2' })).not.toBeInTheDocument()
  })

  it('shows error when createDevice fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createDevice).mockRejectedValue(new Error('name taken'))

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    await user.click(screen.getByTitle('Add Device'))
    await user.type(screen.getByPlaceholderText('PLC 300'), 'Bad')
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(screen.getByText('name taken')).toBeInTheDocument()
    })
  })

  it('closes modal on cancel without calling API', async () => {
    const user = userEvent.setup()
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    await user.click(screen.getByTitle('Add Device'))
    await user.click(screen.getByRole('button', { name: /cancel/i }))

    expect(screen.queryByText('Add Device', { selector: 'h2' })).not.toBeInTheDocument()
    expect(api.createDevice).not.toHaveBeenCalled()
  })
})

describe('AppShell — device start/stop/delete', () => {
  it('calls startDevice when play button is clicked on stopped device', async () => {
    const user = userEvent.setup()
    vi.mocked(api.startDevice).mockResolvedValue({ ...dev1, status: 'running' })

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    const deviceItem = screen.getByText('PLC300').closest('div[class*="group"]')!
    await user.hover(deviceItem)
    await user.click(screen.getByTitle('Start'))

    await waitFor(() => {
      expect(api.startDevice).toHaveBeenCalledWith('d1')
    })
  })

  it('calls stopDevice when stop button is clicked on running device', async () => {
    vi.mocked(api.listDevices).mockResolvedValue([{ ...dev1, status: 'running' }])
    vi.mocked(api.stopDevice).mockResolvedValue({ ...dev1, status: 'stopped' })
    const user = userEvent.setup()

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    const deviceItem = screen.getByText('PLC300').closest('div[class*="group"]')!
    await user.hover(deviceItem)
    await user.click(screen.getByTitle('Stop'))

    await waitFor(() => {
      expect(api.stopDevice).toHaveBeenCalledWith('d1')
    })
  })

  it('calls deleteDevice after confirmation', async () => {
    const user = userEvent.setup()
    vi.mocked(api.deleteDevice).mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    const deviceItem = screen.getByText('PLC300').closest('div[class*="group"]')!
    await user.hover(deviceItem)
    await user.click(screen.getByTitle('Delete'))

    await waitFor(() => {
      expect(api.deleteDevice).toHaveBeenCalledWith('d1')
    })
  })

  it('does not delete when confirm is cancelled', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(false)

    renderApp()
    await waitFor(() => screen.getByText('PLC300'))

    const deviceItem = screen.getByText('PLC300').closest('div[class*="group"]')!
    await user.hover(deviceItem)
    await user.click(screen.getByTitle('Delete'))

    expect(api.deleteDevice).not.toHaveBeenCalled()
  })
})

describe('AppShell — navigation', () => {
  it('renders Dashboard nav link', async () => {
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))
    expect(screen.getByText('Dashboard').closest('a')).toBeInTheDocument()
  })

  it('renders Registers and Versions nav links', async () => {
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))
    expect(screen.getByText('Registers').closest('a')).toBeInTheDocument()
    expect(screen.getByText('Versions').closest('a')).toBeInTheDocument()
  })

  it('shows "select or create a device" placeholder when nothing is selected', async () => {
    vi.mocked(api.listDevices).mockResolvedValue([])
    renderApp()
    await waitFor(() => {
      expect(screen.getByText(/select or create a device/i)).toBeInTheDocument()
    })
  })
})

describe('AppShell — connection status', () => {
  it('shows "Disconnected" when not connected', async () => {
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))
    expect(screen.getByText('Disconnected')).toBeInTheDocument()
  })
})

describe('DeviceItem — display', () => {
  it('shows modbus address for each device', async () => {
    vi.mocked(api.listDevices).mockResolvedValue([dev1, dev2])
    renderApp()
    await waitFor(() => screen.getByText(':5020'))
    expect(screen.getByText(':5021')).toBeInTheDocument()
  })

  it('selected device has highlighted background', async () => {
    renderApp()
    await waitFor(() => screen.getByText('PLC300'))
    const deviceItem = screen.getByText('PLC300').closest('div[class*="group"]')
    expect(deviceItem?.className).toMatch(/bg-slate-700/)
  })
})
