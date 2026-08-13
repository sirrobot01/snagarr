import type { ReactNode } from 'react';
import type { ServiceKey } from '../../lib/types';
import type { Draft } from './draft';
import { cardStatus, useSaveThenTest, type CardStatus } from './service';

interface HeadProps {
  name: string;
  configured: boolean;
  state: CardStatus['state'];
  label: string;
}

export function CardHead({ name, configured, state, label }: HeadProps) {
  return (
    <div className="flex items-start gap-2">
      <span className="sg-dot mt-[6px]" data-state={state} />
      <span className="sg-card-name" data-unset={configured ? undefined : '1'}>
        {name}
      </span>
      <span className="sg-k ml-auto text-right">{label}</span>
    </div>
  );
}

interface Props {
  service: ServiceKey;
  name: string;
  configured: boolean;
  draft: Draft;
  children: ReactNode;
}

export function ServiceCard({ service, name, configured, draft, children }: Props) {
  const test = useSaveThenTest(service, draft);
  const status = cardStatus(configured, test.result);
  const failed = test.result?.ok === false;

  return (
    <section className="sg-card">
      <CardHead
        name={name}
        configured={configured}
        state={status.state}
        label={test.pending ? 'TESTING…' : status.label}
      />

      {children}

      <button
        type="button"
        className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px] self-start`}
        style={{ fontSize: 12 }}
        disabled={test.pending}
        onClick={test.run}
      >
        {failed ? 'RETEST' : 'Test connection'}
      </button>
    </section>
  );
}
