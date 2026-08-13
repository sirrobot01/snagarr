import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { ApiError, api } from '../../lib/api';
import { useStatus, useUsers } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser, ShortcutLink } from '../../lib/types';
import { AddMemberForm } from './AddMemberForm';
import { BookmarkletPanel, useBookmarklet } from './Bookmarklet';
import { HouseholdTable } from './HouseholdTable';
import { MemberServices } from './MemberServices';
import { ShortcutPanel } from './ShortcutPanel';
import { ErrorState, Loading, errorText } from './states';

interface Issued {
  user: HouseholdUser;
  link: ShortcutLink;
}

export function HouseholdSection({ publicUrl, meId }: { publicUrl: string; meId: number }) {
  const users = useUsers(true);
  const status = useStatus();
  const [adding, setAdding] = useState(false);
  const [issued, setIssued] = useState<Issued | null>(null);
  const [inspecting, setInspecting] = useState<HouseholdUser | null>(null);
  const bookmarklet = useBookmarklet(publicUrl, meId);

  const sync = useMutation({
    mutationFn: api.sync,
    onSuccess: () => pushToast('RECONCILE STARTED'),
    onError: (error) => pushToast(`RECONCILE FAILED — ${errorText(error).toUpperCase()}`),
  });

  const shortcut = useMutation({
    mutationFn: (user: HouseholdUser) =>
      api.shortcut(user.id).then((link) => ({ user, link })),
    onSuccess: setIssued,
    onError: (error) =>
      pushToast(
        error instanceof ApiError && error.status === 404
          ? 'SHORTCUT LINKS ARE NOT AVAILABLE ON THIS SERVER YET'
          : `SHORTCUT FAILED — ${errorText(error).toUpperCase()}`,
      ),
  });

  const running = status.data?.sync.running === true || sync.isPending;

  return (
    <section className="sg-pad flex flex-col gap-4 py-6">
      <h4 className="m-0">Household &amp; tokens</h4>

      {users.isError ? (
        <ErrorState error={users.error} onRetry={() => void users.refetch()} />
      ) : users.data ? (
        <HouseholdTable
          users={users.data.users}
          meId={meId}
          busy={shortcut.isPending}
          onShortcut={(user) => shortcut.mutate(user)}
          onInspect={setInspecting}
        />
      ) : (
        <Loading label="LOADING HOUSEHOLD…" />
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          onClick={() => setAdding(true)}
        >
          Add household member
        </button>
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          disabled={bookmarklet.pending}
          onClick={bookmarklet.generate}
        >
          {bookmarklet.pending ? 'GENERATING…' : 'Generate bookmarklet'}
        </button>
        <button
          type="button"
          className="btn btn-ghost ml-auto min-h-[44px]"
          disabled={running}
          onClick={() => sync.mutate()}
        >
          {running ? 'Reconciling…' : 'Force reconcile now'}
        </button>
      </div>

      {adding && <AddMemberForm onDone={() => setAdding(false)} />}
      {inspecting && (
        <MemberServices user={inspecting} onClose={() => setInspecting(null)} />
      )}
      {issued && (
        <ShortcutPanel name={issued.user.display_name} link={issued.link} publicUrl={publicUrl} />
      )}
      {bookmarklet.code !== null && <BookmarkletPanel code={bookmarklet.code} />}
    </section>
  );
}
