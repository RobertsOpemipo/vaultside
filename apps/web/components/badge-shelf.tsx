import type { BadgeItem } from "@/lib/types";

function medalColor(category: BadgeItem["category"], earned: boolean) {
  if (!earned) return "border-ink/10 bg-paper text-ink-muted";
  switch (category) {
    case "milestone":
      return "border-marigold/40 bg-marigold/15 text-amber-text";
    case "skill":
      return "border-cobalt/30 bg-cobalt/10 text-cobalt";
    case "behavior":
      return "border-moss/30 bg-moss/10 text-moss";
    case "team":
      return "border-harbour/30 bg-harbour/10 text-harbour";
  }
}

export function BadgeShelf({ badges }: { badges: BadgeItem[] }) {
  const earned = badges.filter((b) => b.earned).length;
  return (
    <section className="rounded-[var(--radius-card)] bg-card p-6 shadow-[var(--shadow-paper)]">
      <header className="flex items-baseline justify-between">
        <h2 className="font-display text-2xl text-ink">Badges</h2>
        <p className="font-mono text-sm text-ink-muted">
          {earned} / {badges.length}
        </p>
      </header>
      <ul className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {badges.map((b) => (
          <li
            key={b.key}
            className={`flex flex-col items-center rounded-[var(--radius-card)] border p-4 text-center transition-transform duration-[var(--duration-fast)] ${medalColor(
              b.category,
              b.earned,
            )}`}
          >
            <div
              className="flex size-12 items-center justify-center rounded-full border border-current/20 font-display text-lg"
              aria-hidden
            >
              {b.name.charAt(0)}
            </div>
            <p className="mt-3 text-sm font-medium">{b.name}</p>
            <p className="mt-1 text-xs leading-tight opacity-80">
              {b.description}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}