CREATE TABLE project(
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL UNIQUE
);

CREATE TABLE task(
	id INTEGER PRIMARY KEY,

	title TEXT NOT NULL,

	created_sec INTEGER NOT NULL,
	last_completed_sec INTEGER DEFAULT 0,
	last_rejected_sec INTEGER DEFAULT 0,

	-- STATES:
	--     - 0: Active
	--     - 1: Completed
	--     - 2: Rejected
	state INTEGER DEFAULT 0,

	-- PRIORITIES:
	--     - 0: sometime later
	--     - 1: this week
	--     - 2: today
	-- Priority should be updated for scheduled tasks according to current time.
	-- It should be forbidden for external modification, if the schedule is set.
	completion_priority INTEGER DEFAULT 0,
	-- Time in which this task should be done.
	-- Format: YYYY[-MM[-DD]] [HH:mm:ss], so year should always be set if the schedule is set.
	--                                    It also must be UTC.
	schedule TEXT DEFAULT NULL,

	project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE DEFAULT 1
);