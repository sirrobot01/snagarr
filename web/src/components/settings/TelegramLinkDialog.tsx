import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Send } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../lib/api';
import { keys } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser } from '../../lib/types';
import { Modal } from '../Modal';
import { TextField } from './fields';
import { errorText } from './states';

const FORM_ID = 'telegram-link-form';

/* Links one member — the admin's own row included — to a Telegram account.
   The bot tells anyone who messages it their ID, which is what goes here.
   An empty field unlinks. */
export function TelegramLinkDialog({
  user,
  onClose,
}: {
  user: HouseholdUser;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [telegram, setTelegram] = useState(
    user.telegram_user_id === null ? '' : String(user.telegram_user_id),
  );
  const id = Number(telegram.trim());
  const valid = telegram.trim() === '' || (Number.isFinite(id) && id > 0);

  const save = useMutation({
    mutationFn: () =>
      api.updateUser(user.id, { telegram_user_id: telegram.trim() === '' ? 0 : id }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: keys.users });
      pushToast(telegram.trim() === '' ? 'Telegram unlinked' : `Linked Telegram for ${user.username}`);
      onClose();
    },
    onError: (error) => pushToast(`Link failed — ${errorText(error)}`),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={`Link Telegram for ${user.username}`}
      description="Captures the bot receives from this Telegram account land with this member's name on them."
      footer={
        <>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button
            type="submit"
            form={FORM_ID}
            className="btn btn-primary ml-auto"
            disabled={save.isPending || !valid}
          >
            <Send aria-hidden="true" size={16} />
            {save.isPending ? 'Saving…' : 'Save'}
          </button>
        </>
      }
    >
      <form
        id={FORM_ID}
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          if (valid) save.mutate();
        }}
      >
        <TextField
          id="telegram-link-id"
          label="Telegram user ID"
          value={telegram}
          inputMode="numeric"
          description="Message the household bot once and it replies with this number. Leave empty to unlink."
          onChange={setTelegram}
        />
      </form>
    </Modal>
  );
}
