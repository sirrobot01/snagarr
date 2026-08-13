import { CheckCircle2, PlugZap } from 'lucide-react';
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
        <PlugZap aria-hidden="true" size={16} />
        {test.pending ? 'Testing…' : failed ? 'Retry' : 'Test connection'}
      </button>

      {result?.ok === true && (
        <p className="sg-success m-0 flex items-center gap-2 p-3">
          <CheckCircle2 aria-hidden="true" size={16} />
          {note ?? result.message}
        </p>
      )}
      {failed && <p className="sg-k sg-error m-0">{result.message}</p>}
    </div>
  );
}
