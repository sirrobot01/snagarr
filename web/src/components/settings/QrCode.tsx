import { useMemo } from 'react';
import { encodeQr, qrPath } from '../../lib/qr';

/** Renders text as a QR matrix in one SVG path. The quiet zone is part of the
    symbol: without four light modules around it, a camera cannot find it. */
export function QrCode({ text, label, size = 176 }: { text: string; label: string; size?: number }) {
  const qr = useMemo(() => encodeQr(text), [text]);

  if (!qr) {
    return <p className="sg-k m-0">LINK TOO LONG FOR A QR CODE — USE THE LINK ABOVE.</p>;
  }

  const quiet = 4;
  const span = qr.size + quiet * 2;

  return (
    <svg
      role="img"
      aria-label={label}
      width={size}
      height={size}
      viewBox={`0 0 ${span} ${span}`}
      shapeRendering="crispEdges"
      style={{ background: '#fff', flex: 'none' }}
    >
      <g transform={`translate(${quiet} ${quiet})`} fill="#000">
        <path d={qrPath(qr)} />
      </g>
    </svg>
  );
}
