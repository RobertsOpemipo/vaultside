export type Me = {
  user_id: string;
  email: string;
  full_name: string;
  tenant_id: string;
  tenant_slug: string;
  status: string;
  roles: string[];
  permissions: string[];
};

export type XPSummary = {
  level: number;
  total_xp: number;
  xp_into_level: number;
  xp_for_next_level: number;
  progress_pct: number;
};

export type Streak = {
  current_days: number;
  longest_days: number;
  last_activity_on?: string;
};

export type QuestItem = {
  id: string;
  key: string;
  title: string;
  description?: string;
  cadence: "daily" | "weekly" | "onboarding" | "mission";
  kind: string;
  target_value: number;
  progress: number;
  xp_reward: number;
  expires_at?: string;
  completed_at?: string;
};

export type QuestsResponse = {
  active: QuestItem[];
  completed: QuestItem[];
};

export type BadgeItem = {
  key: string;
  name: string;
  description: string;
  category: "milestone" | "skill" | "behavior" | "team";
  points_value: number;
  theme_meta?: { icon?: string; color?: string };
  awarded_at?: string;
  earned: boolean;
};

export type BadgesResponse = {
  badges: BadgeItem[];
};