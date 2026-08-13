import { useMutation, useQueryClient } from '@tanstack/react-query';
import { UserPlus } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../lib/api';
import { keys } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Role } from '../../lib/types';
import { Modal } from '../Modal';
import { Seg, TextField } from './fields';
import { errorText } from './states';

const ROLES: { value: Role; label: string }[] = [
  { value: 'member', label: 'Member' },
  { value: 'admin', label: 'Admin' },
];

const FORM_ID = 'member-form';

export function MemberDialog({ onClose }: { onClose: () => void }) {
  const client = useQueryClient();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<Role>('member');
  const [telegram, setTelegram] = useState('');
  const valid = username.trim() !== '' && password !== '';

  const create = useMutation({
    mutationFn: () => {
      const id = Number(telegram.trim());
      return api.createUser({
        username: username.trim(),
        password,
        role,
        telegram_user_id: telegram.trim() !== '' && Number.isFinite(id) ? id : undefined,
      });
    },
    onSuccess: (user) => {
      void client.invalidateQueries({ queryKey: keys.users });
      pushToast(`Added ${user.username}`);
      onClose();
    },
    onError: (error) => pushToast(`Add failed — ${errorText(error)}`),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title="Add a household member"
      description="Give them sign-in details now. They connect their own services later."
      footer={
        <>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button
            type="submit"
            form={FORM_ID}
            className="btn btn-primary ml-auto"
            disabled={create.isPending || !valid}
          >
            <UserPlus aria-hidden="true" size={16} />
            {create.isPending ? 'Adding…' : 'Add member'}
          </button>
        </>
      }
    >
      <form
        id={FORM_ID}
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          if (valid) create.mutate();
        }}
      >
        <div className="sg-field-grid">
          <TextField
            id="member-username"
            label="Username"
            value={username}
            autoComplete="username"
            description="Used to sign in. Keep it short and easy to remember."
            required
            onChange={setUsername}
          />
          <TextField
            id="member-password"
            label="Password"
            value={password}
            type="password"
            autoComplete="new-password"
            required
            onChange={setPassword}
          />
          <TextField
            id="member-telegram"
            label="Telegram user ID"
            value={telegram}
            inputMode="numeric"
            description="Optional. Links captures from your Telegram bot to this person."
            onChange={setTelegram}
          />
        </div>
        <div className="field">
          <label>Account role</label>
          <Seg name="member-role" value={role} options={ROLES} onChange={setRole} />
        </div>
      </form>
    </Modal>
  );
}
