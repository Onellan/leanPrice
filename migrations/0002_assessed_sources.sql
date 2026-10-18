INSERT INTO retailers (id, name) VALUES
    ('retailer-takealot', 'Takealot'),
    ('retailer-clicks', 'Clicks'),
    ('retailer-makro', 'Makro'),
    ('retailer-checkers', 'Checkers'),
    ('retailer-dischem', 'Dis-Chem');

INSERT INTO sources (id, retailer_id, source_key, base_host, capability_json, assessment_state,
    assessment_ref, assessment_at, enabled, min_interval_ms, parser_version, created_at, updated_at)
VALUES
    ('source-takealot', 'retailer-takealot', 'takealot', 'takealot.com', '{"manual_url":false,"search":false}', 'disallowed', 'docs/evidence/sources/takealot-assessment.md', 1792281600000, 0, 10000, '0', 1792281600000, 1792281600000),
    ('source-clicks', 'retailer-clicks', 'clicks', 'clicks.co.za', '{"manual_url":false,"search":false}', 'review_required', 'docs/evidence/sources/clicks-assessment.md', 1792281600000, 0, 10000, '0', 1792281600000, 1792281600000),
    ('source-makro', 'retailer-makro', 'makro', 'makro.co.za', '{"manual_url":false,"search":false}', 'review_required', 'docs/evidence/sources/makro-assessment.md', 1792281600000, 0, 10000, '0', 1792281600000, 1792281600000),
    ('source-checkers', 'retailer-checkers', 'checkers', 'checkers.co.za', '{"manual_url":false,"search":false}', 'review_required', 'docs/evidence/sources/checkers-assessment.md', 1792281600000, 0, 10000, '0', 1792281600000, 1792281600000),
    ('source-dischem', 'retailer-dischem', 'dischem', 'dischem.co.za', '{"manual_url":false,"search":false}', 'review_required', 'docs/evidence/sources/dischem-assessment.md', 1792281600000, 0, 10000, '0', 1792281600000, 1792281600000);
