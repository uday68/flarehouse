import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { GuardrailPanel } from './GuardrailPanel';
import type { QueryResponse } from '../types';

describe('GuardrailPanel Component', () => {
  it('renders nothing when query is not blocked', () => {
    const normalResult: QueryResponse = {
      columns: ['status_code', 'count'],
      data: [[200, 1500]],
      rows: 1,
      duration_ms: 2.5,
      cached: false,
      query_blocked: false,
    };

    const { container } = render(<GuardrailPanel queryResult={normalResult} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when queryResult is null', () => {
    const { container } = render(<GuardrailPanel queryResult={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders warning alert with error details when query is blocked', () => {
    const blockedResult: QueryResponse = {
      query_blocked: true,
      guardrail_error:
        'Guardrail Violation: Missing LIMIT: Raw scan query on telemetry dataset lacks a LIMIT clause. Add LIMIT 1000 or an aggregation to protect client memory buffers.',
    };

    render(<GuardrailPanel queryResult={blockedResult} />);

    // Assert alert container is visible
    const alert = screen.getByRole('alert');
    expect(alert).toBeInTheDocument();

    // Assert heading
    expect(
      screen.getByText('Query Blocked by Platform Guardrails'),
    ).toBeInTheDocument();

    // Assert error text
    expect(
      screen.getByText(/Missing LIMIT: Raw scan query on telemetry dataset/i),
    ).toBeInTheDocument();
  });

  it('triggers onAskDoctor callback when clicking "Ask AI Doctor to rewrite this safely"', async () => {
    const user = userEvent.setup();
    const handleAskDoctor = vi.fn();

    const blockedResult: QueryResponse = {
      query_blocked: true,
      guardrail_error: 'Guardrail Violation: Unbounded partition scan',
    };

    render(
      <GuardrailPanel
        queryResult={blockedResult}
        onAskDoctor={handleAskDoctor}
      />,
    );

    const button = screen.getByRole('button', {
      name: /Ask AI Doctor to rewrite this safely/i,
    });
    expect(button).toBeInTheDocument();

    await user.click(button);
    expect(handleAskDoctor).toHaveBeenCalledTimes(1);
  });
});
