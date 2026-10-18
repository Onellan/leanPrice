ALTER TABLE watch_rule_versions ADD COLUMN notification_channel TEXT
	CHECK (notification_channel IS NULL OR notification_channel IN ('in_app','webhook'));
