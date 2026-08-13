import type { ReactNode } from 'react';
import type { ServiceKey } from '../../lib/types';
import type { Draft } from './draft';
import { cardStatus, useSaveThenTest } from './service';

interface Props {
  service: ServiceKey;
  name: string;
  configured: boolean;
  draft: Draft;
  children: ReactNode;
  /* POST /settings/test has no probe for every service — Telegram has none. */
  testable?: boolean;
}

export function ServiceCard({ service, name, configured, draft, children, testable = true }: Props) {
  const test = useSaveThenTest(service, draft);
  const status = cardStatus(configured, test.result);
  const failed = test.result?.ok === false;

  return (
    <section className="sg-card">
      <div className="flex items-start gap-2">
        <span className="sg-dot mt-[6px]" data-state={status.state} />
        <span className="sg-card-name" data-unset={configured ? undefined : '1'}>
          {name}
        </span>
        <span className="sg-k ml-auto text-right">
          {test.pending ? 'TESTING…' : status.label}
        </span>
      </div>

      {children}

      {testable && (
        <button
          type="button"
          className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px] self-start`}
          style={{ fontSize: 12 }}
          disabled={test.pending}
          onClick={test.run}
        >
          {failed ? 'RETEST' : 'Test connection'}
        </button>
      )}
    </section>
  );
}
