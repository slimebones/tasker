CREATE TABLE project(
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL UNIQUE
);

CREATE TABLE task(
	id INTEGER PRIMARY KEY,

	title TEXT NOT NULL,
	-- PRIORITIES:
	--     - 1: sometime later
	--     - 2: this week
	--     - 3: today
	-- Priority should be updated for scheduled tasks according to current time.
	-- It should be forbidden for external modification, if the schedule is set.
	completion_priority INTEGER DEFAULT 1,
	-- Time in which this task should be done.
	-- Format: YYYY[-MM[-DD]] [HH:mm:ss AM/PM], so year should always be set if
	--         the schedule is set.
	schedule TEXT DEFAULT NULL,

	project_id INTEGER NOT NULL REFERENCES project(id)
);