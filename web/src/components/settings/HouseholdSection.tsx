import { useMutation } from '@tanstack/react-query';
import { Bookmark, Play, UserPlus, Users } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../lib/api';
import { useStatus, useUsers } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser } from '../../lib/types';
import { AddMemberForm } from './AddMemberForm';
import { BookmarkletPanel, useBookmarklet } from './Bookmarklet';
import { HouseholdTable } from './HouseholdTable';
import { MemberServices } from './MemberServices';
import { ErrorState, Loading, errorText } from './states';

export function HouseholdSection({ publicUrl, meId }: { publicUrl: string; meId: number }) {
  const users = useUsers(true);
  const status = useStatus();
  const [adding, setAdding] = useState(false);
  const [inspecting, setInspecting] = useState<HouseholdUser | null>(null);
  const bookmarklet = useBookmarklet(publicUrl, meId);

  const sync = useMutation({
    mutationFn: api.sync,
    onSuccess: () => pushToast('Reconcile started'),
    onError: (error) => pushToast(`Reconcile failed — ${errorText(error)}`),
  });

  const running = status.data?.sync.running === true || sync.isPending;

  return (
    <section className="sg-household sg-pad flex flex-col gap-4 py-6">
      <div className="sg-section-heading">
        <Users aria-hidden="true" size={20} />
        <div>
          <h3 className="m-0 text-[22px]">Household access</h3>
          <p className="text-muted m-0 text-[13px]">
            Manage who can sign in, their connected services, and capture tokens.
          </p>
        </div>
      </div>

      {users.isError ? (
        <ErrorState error={users.error} onRetry={() => void users.refetch()} />
      ) : users.data ? (
        <HouseholdTable users={users.data.users} meId={meId} onInspect={setInspecting} />
      ) : (
        <Loading label="Loading household…" />
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          onClick={() => setAdding(true)}
        >
          <UserPlus aria-hidden="true" size={16} />
          Add household member
        </button>
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          disabled={bookmarklet.pending}
          onClick={bookmarklet.generate}
        >
          <Bookmark aria-hidden="true" size={16} />
          {bookmarklet.pending ? 'Generating…' : 'Generate bookmarklet'}
        </button>
        <button
          type="button"
          className="btn btn-ghost ml-auto min-h-[44px]"
          disabled={running}
          onClick={() => sync.mutate()}
        >
          <Play aria-hidden="true" size={16} />
          {running ? 'Reconciling…' : 'Force reconcile now'}
        </button>
      </div>

      {adding && <AddMemberForm onDone={() => setAdding(false)} />}
      {inspecting && (
        <MemberServices user={inspecting} onClose={() => setInspecting(null)} />
      )}
      {bookmarklet.code !== null && <BookmarkletPanel code={bookmarklet.code} />}
    </section>
  );
}
