import { posterUrl } from '../lib/format';

interface Props {
  path: string | null;
  title: string;
  size: 'w185' | 'w342';
  /** Fixed pixel box. Omit and pass `fill` or a sizing class instead. */
  width?: number;
  height?: number;
  fill?: boolean;
  className?: string;
}

/* `.sg-p` paints the title as the fallback; the image sits on top of it and
   loads lazily, so a missing or slow poster still reads. */
export function Poster({ path, title, size, width, height, fill, className }: Props) {
  const url = posterUrl(path, size);
  const style = width
    ? { width, height, minWidth: width }
    : fill
      ? { width: '100%', aspectRatio: '2 / 3' }
      : undefined;

  return (
    <div className={`sg-p ${className ?? ''}`} style={style}>
      <span>{title}</span>
      {url && (
        <img
          className="sg-p-img"
          src={url}
          alt=""
          width={width}
          height={height}
          loading="lazy"
          decoding="async"
        />
      )}
    </div>
  );
}
