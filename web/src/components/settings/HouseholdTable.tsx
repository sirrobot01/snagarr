import { useMutation, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Server, Trash2 } from 'lucide-react';
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
  onInspect: (user: HouseholdUser) => void;
}

export function HouseholdTable({ users, meId, onInspect }: Props) {
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
      pushToast(`Revoked ${count} token${count === 1 ? '' : 's'}`);
      refresh();
    },
    onError: (error) => pushToast(`Revoke failed — ${errorText(error)}`),
  });

  const remove = useMutation({
    mutationFn: (userId: number) => api.deleteUser(userId),
    onSuccess: () => {
      pushToast('Member removed');
      refresh();
    },
    onError: (error) => pushToast(`Remove failed — ${errorText(error)}`),
  });

  const busy = revoke.isPending || remove.isPending;

  return (
    <table className="table">
      <thead>
        <tr>
          <th>Username</th>
          <th>Role</th>
          <th>Tokens</th>
          <th />
        </tr>
      </thead>
      <tbody>
        {users.map((user) => (
          <tr key={user.id}>
            <td>
              <span className="block font-heading font-extrabold">@{user.username}</span>
              {user.id === meId && <span className="sg-k">You</span>}
            </td>
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
                onClick={() => onInspect(user)}
              >
                <Server aria-hidden="true" size={15} />
                Services
              </button>
              <button
                type="button"
                className="btn btn-ghost min-h-[44px]"
                disabled={busy || user.token_count === 0}
                onClick={() => {
                  if (window.confirm(`Revoke every token for ${user.username}?`)) {
                    revoke.mutate(user.id);
                  }
                }}
              >
                <KeyRound aria-hidden="true" size={15} />
                Revoke
              </button>
              {user.id !== meId && (
                <button
                  type="button"
                  className="btn btn-ghost min-h-[44px]"
                  disabled={busy}
                  onClick={() => {
                    if (window.confirm(`Remove ${user.username} from the household?`)) {
                      remove.mutate(user.id);
                    }
                  }}
                >
                  <Trash2 aria-hidden="true" size={15} />
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
