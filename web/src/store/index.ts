import { configureStore } from '@reduxjs/toolkit';
import { gatewayApi } from './api';

export const store = configureStore({
  reducer: {
    [gatewayApi.reducerPath]: gatewayApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(gatewayApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
