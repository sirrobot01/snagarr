export function errorText(error: unknown): string {
  return error instanceof Error ? error.message : 'request failed';
}

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return (
    <p className="sg-k m-0 flex items-center gap-2 py-6" role="status">
      <LoaderCircle className="animate-spin" aria-hidden="true" size={14} />
      {label}
    </p>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-start gap-3 py-6">
      <p className="sg-k sg-error m-0 flex items-center gap-2">
        <AlertCircle aria-hidden="true" size={15} />
        {errorText(error)}
      </p>
      <button type="button" className="btn btn-secondary min-h-[44px]" onClick={onRetry}>
        <RotateCw aria-hidden="true" size={15} />
        Retry
      </button>
    </div>
  );
}
import { AlertCircle, LoaderCircle, RotateCw } from 'lucide-react';
