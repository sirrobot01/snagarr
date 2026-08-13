import { AlertTriangle, Bookmark } from 'lucide-react';
import { useState } from 'react';
import { useCreateToken } from '../../lib/queries';
import { Modal } from '../Modal';
import { CopyField } from './fields';

function build(publicUrl: string, token: string): string {
  const base = (publicUrl.trim() === '' ? window.location.origin : publicUrl).replace(/\/+$/, '');
  return (
    `javascript:(function(){fetch('${base}/api/v1/capture',{method:'POST',` +
    `headers:{'Content-Type':'application/json','Authorization':'Bearer ${token}'},` +
    `body:JSON.stringify({url:location.href,source:'bookmarklet'})})` +
    `.then(function(r){alert(r.ok?'Snagged':'Snagarr said '+r.status)})` +
    `.catch(function(e){alert('Snagarr unreachable: '+e)})})()`
  );
}

export function BookmarkletDialog({
  publicUrl,
  userId,
  onClose,
}: {
  publicUrl: string;
  userId: number;
  onClose: () => void;
}) {
  const create = useCreateToken(userId);
  const [code, setCode] = useState<string | null>(null);

  return (
    <Modal
      open
      onClose={onClose}
      title="Browser bookmarklet"
      description="A bookmark that snags the page you are on, in one click."
      footer={
        <>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Close
          </button>
          <button
            type="button"
            className="btn btn-primary ml-auto"
            disabled={create.isPending}
            onClick={() =>
              create.mutate('Bookmarklet', {
                onSuccess: (token) => setCode(build(publicUrl, token.token)),
              })
            }
          >
            <Bookmark aria-hidden="true" size={16} />
            {create.isPending ? 'Generating…' : code ? 'Generate another' : 'Generate link'}
          </button>
        </>
      }
    >
      {code === null ? (
        <p className="text-muted m-0 text-[13px]">
          Generating issues a token named <code>Bookmarklet</code> and writes it into the link.
          Revoke that token to switch the bookmark off.
        </p>
      ) : (
        <div className="sg-reveal">
          <CopyField id="bookmarklet" label="Drag this to your bookmarks bar" value={code} />
          <p className="sg-k m-0 flex items-center gap-2">
            <AlertTriangle aria-hidden="true" size={14} />
            This link carries a working token in plain text. Do not share it.
          </p>
        </div>
      )}
    </Modal>
  );
}
