export function Footer({ context }: { context: string }) {
  return (
    <footer className="sg-foot">
      <span className="sg-foot-item">Metadata by TMDB</span>
      <span className="sg-foot-item">{context}</span>
    </footer>
  );
}
