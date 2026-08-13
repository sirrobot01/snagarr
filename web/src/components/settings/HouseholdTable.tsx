import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import { keys } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser } from '../../lib/types';
import { errorText } from './states';

function detail(user: HouseholdUser): string {
  const parts = [`${user.token_count} token${user.token_count === 1 ? '' : 's'}`];
  if (user.telegram_user_id !== null) {
    parts.push(`telegram ${String(user.telegram_user_id).slice(0, 4)}…`);
  }
  return parts.join(' · ');
}

interface Props {
  users: HouseholdUser[];
  meId: number;
  busy: boolean;
  onShortcut: (user: HouseholdUser) => void;
  onInspect: (user: HouseholdUser) => void;
}

export function HouseholdTable({ users, meId, busy: pending, onShortcut, onInspect }: Props) {
  const client = useQueryClient();

  const refresh = () => void client.invalidateQueries({ queryKey: keys.users });

  const revoke = useMutation({
    mutationFn: async (userId: number) => {
      const { tokens } = await api.tokens(userId);
      for (const token of tokens.filter((item) => !item.revoked)) {
        await api.revokeToken(token.id);
      }
      return tokens.length;
    },
    onSuccess: (count) => {
      pushToast(`REVOKED — ${count} TOKEN${count === 1 ? '' : 'S'}`);
      refresh();
    },
    onError: (error) => pushToast(`REVOKE FAILED — ${errorText(error).toUpperCase()}`),
  });

  const remove = useMutation({
    mutationFn: (userId: number) => api.deleteUser(userId),
    onSuccess: () => {
      pushToast('MEMBER REMOVED');
      refresh();
    },
    onError: (error) => pushToast(`REMOVE FAILED — ${errorText(error).toUpperCase()}`),
  });

  const busy = revoke.isPending || remove.isPending || pending;

  return (
    <table className="table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Role</th>
          <th>Tokens</th>
          <th />
        </tr>
      </thead>
      <tbody>
        {users.map((user) => (
          <tr key={user.id}>
            <td className="font-heading font-extrabold">{user.display_name}</td>
            <td>
              <span className={`sg-b ${user.role === 'admin' ? 'sg-lib' : 'sg-new'}`}>
                {user.role}
              </span>
            </td>
            <td className="text-muted">{detail(user)}</td>
            <td className="text-right whitespace-nowrap">
              <button
                type="button"
                className="btn btn-ghost min-h-[44px]"
                disabled={busy}
                onClick={() => onShortcut(user)}
              >
                Shortcut
              </button>
              <button
                type="button"
                className="btn btn-ghost min-h-[44px]"
                disabled={busy}
                onClick={() => onInspect(user)}
              >
                Services
              </button>
              <button
                type="button"
                className="btn btn-ghost min-h-[44px]"
                disabled={busy || user.token_count === 0}
                onClick={() => {
                  if (window.confirm(`Revoke every token for ${user.display_name}?`)) {
                    revoke.mutate(user.id);
                  }
                }}
              >
                Revoke
              </button>
              {user.id !== meId && (
                <button
                  type="button"
                  className="btn btn-ghost min-h-[44px]"
                  disabled={busy}
                  onClick={() => {
                    if (window.confirm(`Remove ${user.display_name} from the household?`)) {
                      remove.mutate(user.id);
                    }
                  }}
                >
                  Remove
                </button>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
