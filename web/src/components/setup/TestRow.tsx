import type { ServiceTest } from '../settings/service';

export function TestRow({ test, note }: { test: ServiceTest; note?: string }) {
  const result = test.result;
  const failed = result?.ok === false;

  return (
    <div className="flex flex-col gap-3">
      <button
        type="button"
        className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px] self-start`}
        style={{ fontSize: 12 }}
        disabled={test.pending}
        onClick={test.run}
      >
        {test.pending ? 'TESTING…' : failed ? 'RETEST' : 'Test connection'}
      </button>

      {result?.ok === true && (
        <p className="sg-note m-0">{note ?? result.message.toUpperCase()}</p>
      )}
      {failed && <p className="sg-k m-0">{result.message.toUpperCase()}</p>}
    </div>
  );
}
