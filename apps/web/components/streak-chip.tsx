import type { Streak } from "@/lib/types";

export function StreakChip({ streak }: { streak: Streak }) {
  const days = streak.current_days ?? 0;
  return (
    <div
      className="inline-flex items-center gap-2 rounded-full border border-ink/10 bg-card px-4 py-2"
      aria-label={`Current streak: ${days} days`}
    >
      <svg
        viewBox="0 0 24 24"
        className="size-5 text-vermilion"
        aria-hidden
        fill="currentColor"
      >
        <path d="M12 2c1 4 4 6 4 10a4 4 0 0 1-8 0c0-2 1-3 1-3s-3 1-3 4a6 6 0 1 0 12 0c0-5-6-8-6-11Z" />
      </svg>
      <span className="font-mono text-lg text-ink">{days}</span>
      <span className="text-sm text-ink-muted">
        {days === 1 ? "day" : "days"}
      </span>
      {streak.longest_days > days && (
        <span className="text-xs text-ink-muted">· best {streak.longest_days}</span>
      )}
    </div>
  );
}