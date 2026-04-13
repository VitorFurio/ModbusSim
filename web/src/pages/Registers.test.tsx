import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSimStore } from '../store'
import type { Register } from '../types'
import Registers from './Registers'

vi.mock('../api', () => ({
  listRegisters: vi.fn(),
  createRegister: vi.fn(),
  updateRegister: vi.fn(),
  deleteRegister: vi.fn(),
}))

import * as api from '../api'

const mockReg: Register = {
  id: 'r1',
  name: 'Temperature',
  description: 'Sensor temp',
  address: 28040,
  data_type: 'float32',
  unit: '°C',
  signal: { kind: 'constant', value: 25.5 },
  value: 25.5,
  updated_at: 0,
}

beforeEach(() => {
  vi.clearAllMocks()
  useSimStore.setState({
    devices: [{ id: 'd1', name: 'PLC', modbus_addr: ':5020', status: 'running', reg_count: 1 }],
    selectedDeviceId: 'd1',
    registers: [mockReg],
    connected: true,
  })
  vi.mocked(api.listRegisters).mockResolvedValue([mockReg])
})

// Helper: find the Name input in the register modal (second textbox: ID, Name, ...)
function getNameInputInModal() {
  // Order of textbox inputs in RegisterModal: [0]=id, [1]=name, [2]=description, [3]=unit
  return screen.getAllByRole('textbox')[1]
}

describe('Registers page — no device selected', () => {
  it('shows prompt when no device is selected', () => {
    useSimStore.setState({ selectedDeviceId: null })
    render(<Registers />)
    expect(screen.getByText(/select a device/i)).toBeInTheDocument()
  })
})

describe('Registers page — with device', () => {
  it('renders the register table with address, name, type and value', () => {
    render(<Registers />)
    expect(screen.getByText('Temperature')).toBeInTheDocument()
    expect(screen.getByText('28040')).toBeInTheDocument()
    expect(screen.getByText('float32')).toBeInTheDocument()
    expect(screen.getByText('25.50')).toBeInTheDocument()
    expect(screen.getByText('°C')).toBeInTheDocument()
  })

  it('shows "Add Register" button', () => {
    render(<Registers />)
    expect(screen.getByRole('button', { name: /add register/i })).toBeInTheDocument()
  })

  it('opens create modal when "Add Register" is clicked', async () => {
    const user = userEvent.setup()
    render(<Registers />)
    await user.click(screen.getByRole('button', { name: /add register/i }))
    expect(screen.getByText('Add Register', { selector: 'h2' })).toBeInTheDocument()
  })

  it('closes the modal on cancel', async () => {
    const user = userEvent.setup()
    render(<Registers />)
    await user.click(screen.getByRole('button', { name: /add register/i }))
    await user.click(screen.getByRole('button', { name: /cancel/i }))
    expect(screen.queryByText('Add Register', { selector: 'h2' })).not.toBeInTheDocument()
  })

  it('calls createRegister and closes modal on save', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRegister).mockResolvedValue({ ...mockReg, id: 'r2', name: 'Pressure' })
    vi.mocked(api.listRegisters).mockResolvedValue([mockReg, { ...mockReg, id: 'r2', name: 'Pressure' }])

    render(<Registers />)
    await user.click(screen.getByRole('button', { name: /add register/i }))

    const nameInput = getNameInputInModal()
    await user.clear(nameInput)
    await user.type(nameInput, 'Pressure')

    await user.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(api.createRegister).toHaveBeenCalledWith('d1', expect.objectContaining({ name: 'Pressure' }))
    })
    // Modal closes on success
    await waitFor(() => {
      expect(screen.queryByText('Add Register', { selector: 'h2' })).not.toBeInTheDocument()
    })
  })

  it('shows error inside modal when createRegister fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRegister).mockRejectedValue(new Error('server error'))

    render(<Registers />)
    await user.click(screen.getByRole('button', { name: /add register/i }))

    const nameInput = getNameInputInModal()
    await user.clear(nameInput)
    await user.type(nameInput, 'Fail')

    await user.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(screen.getByText('server error')).toBeInTheDocument()
    })
    // Modal stays open on error
    expect(screen.getByText('Add Register', { selector: 'h2' })).toBeInTheDocument()
  })

  it('opens edit modal pre-filled with existing register data', async () => {
    const user = userEvent.setup()
    render(<Registers />)
    await user.click(screen.getByTitle('Edit'))
    expect(screen.getByText('Edit Register', { selector: 'h2' })).toBeInTheDocument()
    expect(screen.getByDisplayValue('Temperature')).toBeInTheDocument()
    expect(screen.getByDisplayValue('28040')).toBeInTheDocument()
  })

  it('calls updateRegister when edit form is submitted', async () => {
    const user = userEvent.setup()
    vi.mocked(api.updateRegister).mockResolvedValue({ ...mockReg, name: 'Temp Updated' })
    vi.mocked(api.listRegisters).mockResolvedValue([{ ...mockReg, name: 'Temp Updated' }])

    render(<Registers />)
    await user.click(screen.getByTitle('Edit'))

    const nameInput = screen.getByDisplayValue('Temperature')
    await user.clear(nameInput)
    await user.type(nameInput, 'Temp Updated')
    await user.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(api.updateRegister).toHaveBeenCalledWith('d1', 'r1', expect.objectContaining({ name: 'Temp Updated' }))
    })
  })

  it('calls deleteRegister when delete is confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(api.deleteRegister).mockResolvedValue(undefined)
    vi.mocked(api.listRegisters).mockResolvedValue([])
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(<Registers />)
    await user.click(screen.getByTitle('Delete'))

    await waitFor(() => {
      expect(api.deleteRegister).toHaveBeenCalledWith('d1', 'r1')
    })
  })

  it('does not call deleteRegister when confirm is cancelled', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(false)

    render(<Registers />)
    await user.click(screen.getByTitle('Delete'))

    expect(api.deleteRegister).not.toHaveBeenCalled()
  })

  it('shows empty state when no registers', () => {
    useSimStore.setState({ registers: [] })
    render(<Registers />)
    expect(screen.getByText(/no registers/i)).toBeInTheDocument()
  })
})
