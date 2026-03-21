import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import MessageBubble from '../MessageBubble';

describe('MessageBubble', () => {
  it('renders user message', () => {
    render(<MessageBubble message={{
      id: '1', role: 'user', content: 'Hello world',
      createdAt: new Date().toISOString()
    }} />);
    expect(screen.getByText('Hello world')).toBeInTheDocument();
  });

  it('renders assistant message', () => {
    render(<MessageBubble message={{
      id: '2', role: 'assistant', content: 'Hi there!',
      createdAt: new Date().toISOString()
    }} />);
    expect(screen.getByText('Hi there!')).toBeInTheDocument();
  });

  it('renders code blocks', async () => {
    render(<MessageBubble message={{
      id: '3', role: 'assistant', content: '```python\nprint("hello")\n```',
      createdAt: new Date().toISOString()
    }} />);
    expect(await screen.findByText(/print/)).toBeInTheDocument();
  });
});
