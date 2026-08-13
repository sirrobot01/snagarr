export function errorText(error: unknown): string {
  return error instanceof Error ? error.message : 'request failed';
}

export function Loading({ label = 'LOADING…' }: { label?: string }) {
  return <p className="sg-k m-0 py-6">{label}</p>;
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-start gap-3 py-6">
      <p className="sg-k m-0">{errorText(error).toUpperCase()}</p>
      <button type="button" className="btn btn-secondary min-h-[44px]" onClick={onRetry}>
        Retry
      </button>
    </div>
  );
}
