import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import StatusBar from '../StatusBar';

describe('StatusBar', () => {
  it('renders token counts', () => {
    render(<StatusBar tokensIn={100} tokensOut={50} latencyMs={320} />);
    expect(screen.getByText(/100/)).toBeInTheDocument();
    expect(screen.getByText(/50/)).toBeInTheDocument();
    expect(screen.getByText(/320/)).toBeInTheDocument();
  });

  it('renders zero state (returns null)', () => {
    const { container } = render(<StatusBar tokensIn={0} tokensOut={0} latencyMs={0} />);
    expect(container.firstChild).toBeNull();
  });
});
