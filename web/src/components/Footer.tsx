export function Footer({ context }: { context: string }) {
  return (
    <footer className="sg-foot">
      <span className="sg-k">POWERED BY TMDB</span>
      <span className="sg-k">{context}</span>
    </footer>
  );
}
