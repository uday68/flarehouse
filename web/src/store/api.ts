import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import { API_BASE } from '../config';
import type {
  HealthStatus,
  IngestStatsData,
  PartitionInfo,
  QueryRequest,
  QueryResponse,
  AIDoctorAnalysis,
} from '../types';

export const gatewayApi = createApi({
  reducerPath: 'gatewayApi',
  baseQuery: fetchBaseQuery({
    baseUrl: API_BASE,
  }),
  tagTypes: ['Stats', 'Health', 'Partitions'],
  endpoints: (builder) => ({
    getHealth: builder.query<HealthStatus, void>({
      query: () => '/health',
      providesTags: ['Health'],
    }),
    getStats: builder.query<IngestStatsData, void>({
      query: () => '/api/stats',
      providesTags: ['Stats'],
    }),
    getPartitions: builder.query<PartitionInfo[], void>({
      query: () => '/api/partitions',
      providesTags: ['Partitions'],
    }),
    runQuery: builder.mutation<QueryResponse, QueryRequest>({
      query: (body) => ({
        url: '/api/query',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
      }),
    }),
    explainQuery: builder.mutation<AIDoctorAnalysis, { sql: string }>({
      query: (body) => ({
        url: '/api/explain',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
      }),
    }),
    startLoad: builder.mutation<{ status: string; events: number; message: string }, number | void>({
      query: (count = 25000) => ({
        url: `/api/load/start?count=${count}`,
        method: 'POST',
      }),
      invalidatesTags: ['Stats'],
    }),
    stopLoad: builder.mutation<{ status: string }, void>({
      query: () => ({
        url: '/api/load/stop',
        method: 'POST',
      }),
      invalidatesTags: ['Stats'],
    }),
  }),
});

export const {
  useGetHealthQuery,
  useGetStatsQuery,
  useGetPartitionsQuery,
  useRunQueryMutation,
  useExplainQueryMutation,
  useStartLoadMutation,
  useStopLoadMutation,
} = gatewayApi;
