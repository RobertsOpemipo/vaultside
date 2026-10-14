import type { QuestItem } from "@/lib/types";

const cadenceLabel: Record<QuestItem["cadence"], string> = {
  daily: "Today",
  weekly: "This week",
  onboarding: "Onboarding",
  mission: "Team mission",
};

export function QuestCard({ quest }: { quest: QuestItem }) {
  const pct = Math.min(
    100,
    Math.round((quest.progress / Math.max(1, quest.target_value)) * 100),
  );
  const complete = quest.completed_at != null;

  return (
    <article className="relative rounded-[var(--radius-card)] bg-card p-6 shadow-[var(--shadow-paper)]">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-widest text-ink-muted">
            {cadenceLabel[quest.cadence]}
          </p>
          <h3 className="mt-1 font-display text-2xl text-ink">{quest.title}</h3>
          {quest.description && (
            <p className="mt-2 text-sm text-ink-muted">{quest.description}</p>
          )}
        </div>
        <span className="shrink-0 rounded-full bg-marigold/20 px-3 py-1 font-mono text-sm text-amber-text">
          +{quest.xp_reward} XP
        </span>
      </div>

      <div className="mt-6 flex items-center gap-4">
        <div
          className="h-1.5 flex-1 overflow-hidden rounded-full bg-paper"
          role="progressbar"
          aria-valuenow={pct}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${quest.title}: ${pct}%`}
        >
          <div
            className={`h-full transition-all duration-[var(--duration-base)] ease-[var(--ease-paper)] ${
              complete ? "bg-moss" : "bg-harbour"
            }`}
            style={{ width: `${pct}%` }}
          />
        </div>
        <span className="font-mono text-sm text-ink-muted">
          {quest.progress}/{quest.target_value}
        </span>
      </div>

      {complete && (
        <p className="mt-3 text-sm text-moss">
          ✓ Completed
        </p>
      )}
    </article>
  );
}