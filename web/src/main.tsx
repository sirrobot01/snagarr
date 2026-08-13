import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import '@fontsource/archivo/400.css';
import '@fontsource/archivo/800.css';
import './styles/preflight.css';
import './styles/modernist.css';
import './styles/app.css';
import './styles/utilities.css';

import { App } from './App';
import { ApiError } from './lib/api';
import { initToken } from './lib/auth';

initToken();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (count, error) => !(error instanceof ApiError) && count < 3,
      retryDelay: (attempt) => Math.min(500 * 2 ** attempt, 15_000),
      refetchOnWindowFocus: false,
    },
    mutations: { retry: 0 },
  },
});

createRoot(document.getElementById('root') as HTMLElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
