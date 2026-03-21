import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import ModelSelector from '../ModelSelector';

// Mock the API
vi.mock('../../api/client', () => ({
  fetchModels: vi.fn().mockResolvedValue([
    { id: 'gpt-4o', owned_by: 'openai' },
    { id: 'claude-sonnet-4-20250514', owned_by: 'anthropic' },
  ]),
}));

describe('ModelSelector', () => {
  it('renders with selected model', async () => {
    render(<ModelSelector selectedModel="gpt-4o" onModelChange={() => {}} />);
    // While loading, shows loading text
    expect(screen.getByText(/Loading models/)).toBeInTheDocument();
  });

  it('renders select element after loading', async () => {
    render(<ModelSelector selectedModel="gpt-4o" onModelChange={() => {}} />);
    const select = await screen.findByRole('combobox');
    expect(select).toBeInTheDocument();
  });

  it('shows available models after loading', async () => {
    render(<ModelSelector selectedModel="gpt-4o" onModelChange={() => {}} />);
    expect(await screen.findByText('gpt-4o')).toBeInTheDocument();
    expect(await screen.findByText('claude-sonnet-4-20250514')).toBeInTheDocument();
  });
});
