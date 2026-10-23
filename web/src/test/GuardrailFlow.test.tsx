import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { gatewayApi } from '../store/api';
import App from '../App';

function renderAppWithStore() {
  const testStore = configureStore({
    reducer: {
      [gatewayApi.reducerPath]: gatewayApi.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(gatewayApi.middleware),
  });

  return render(
    <Provider store={testStore}>
      <App />
    </Provider>,
  );
}

describe('Guardrail Warning User Flow', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      let url = '';
      let bodyText = '';

      if (typeof input === 'string') {
        url = input;
        bodyText = init?.body ? String(init.body) : '';
      } else if (input instanceof URL) {
        url = input.toString();
        bodyText = init?.body ? String(init.body) : '';
      } else if (typeof input === 'object' && 'url' in input) {
        url = input.url;
        bodyText = init?.body
          ? String(init.body)
          : await (input as Request).clone().text().catch(() => '');
      }

      if (url.includes('/health')) {
        return new Response(
          JSON.stringify({
            status: 'healthy',
            service: 'flarehouse-gateway',
            clickhouse: 'healthy',
            redis: 'healthy',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.includes('/api/stats')) {
        return new Response(
          JSON.stringify({
            total_events_received: 1000,
            total_events_flushed: 1000,
            batches_written: 1,
            current_buffer_size: 0,
            buffer_capacity: 100000,
            avg_flush_latency_ms: 10,
            events_per_second: 500,
            active_workers: 4,
            dropped_events: 0,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.includes('/api/partitions')) {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }

      if (url.includes('/api/query')) {
        let body: { sql?: string } = {};
        try {
          body = JSON.parse(bodyText);
        } catch {
          // ignore
        }

        if (body.sql && body.sql.includes('SELECT *')) {
          return new Response(
            JSON.stringify({
              query_blocked: true,
              guardrail_error:
                'Guardrail Violation: Missing LIMIT: Raw scan query on telemetry dataset lacks a LIMIT clause.',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          );
        }

        return new Response(
          JSON.stringify({
            columns: ['status_code', 'count'],
            data: [[200, 10]],
            rows: 1,
            duration_ms: 1.2,
            cached: false,
            query_blocked: false,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.includes('/api/explain')) {
        return new Response(
          JSON.stringify({
            original_sql: 'SELECT * FROM flarehouse.edge_http_logs',
            explain_plan: 'ReadFromNullSource',
            issues_identified: ['SELECT * columnar penalty detected'],
            recommendations: ['Add LIMIT and select explicit columns'],
            optimized_sql:
              'SELECT timestamp, client_ip FROM flarehouse.edge_http_logs LIMIT 100',
            expected_impact: '90% I/O reduction',
            generated_by_ai: false,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      return new Response(JSON.stringify({}), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }) as typeof fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('displays the guardrail warning banner when a blocked query is submitted, and transitions to AI Doctor', async () => {
    const user = userEvent.setup();
    renderAppWithStore();

    // Select the "Columnar Anti-Pattern" preset button
    const antiPatternBtn = screen.getByRole('button', {
      name: /Columnar Anti-Pattern/i,
    });
    await user.click(antiPatternBtn);

    // Click "Run Query"
    const runBtn = screen.getByRole('button', { name: /Run Query/i });
    await user.click(runBtn);

    // Verify guardrail alert appears with error message
    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(
      screen.getByText(/Query Blocked by Platform Guardrails/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Missing LIMIT: Raw scan query on telemetry dataset/i),
    ).toBeInTheDocument();

    // Click "Ask AI Doctor to rewrite this safely"
    const askDoctorBtn = screen.getByRole('button', {
      name: /Ask AI Doctor to rewrite this safely/i,
    });
    await user.click(askDoctorBtn);

    // Verify UI navigated to AI Doctor tab and displayed diagnosed suggestions
    await waitFor(() => {
      expect(
        screen.getByText(/AI Query Doctor & ClickHouse Index Optimizer/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/SELECT \* columnar penalty detected/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Add LIMIT and select explicit columns/i),
      ).toBeInTheDocument();
    });
  });
});
