CREATE TABLE retailers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE CHECK (length(trim(name)) BETWEEN 1 AND 120)
);

CREATE TABLE sources (
    id TEXT PRIMARY KEY,
    retailer_id TEXT NOT NULL REFERENCES retailers(id) ON DELETE RESTRICT,
    source_key TEXT NOT NULL UNIQUE CHECK (length(source_key) BETWEEN 1 AND 80),
    base_host TEXT NOT NULL CHECK (length(base_host) BETWEEN 1 AND 253),
    locale TEXT NOT NULL DEFAULT 'en-ZA',
    currency TEXT NOT NULL DEFAULT 'ZAR' CHECK (currency = 'ZAR'),
    capability_json TEXT NOT NULL DEFAULT '{}',
    assessment_state TEXT NOT NULL DEFAULT 'unassessed'
        CHECK (assessment_state IN ('unassessed','review_required','approved','disallowed','unsupported')),
    assessment_ref TEXT,
    assessment_at INTEGER,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
    min_interval_ms INTEGER NOT NULL DEFAULT 10000 CHECK (min_interval_ms >= 10000),
    next_allowed_at INTEGER NOT NULL DEFAULT 0 CHECK (next_allowed_at >= 0),
    parser_version TEXT NOT NULL DEFAULT '0',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (enabled = 0 OR assessment_state = 'approved'),
    CHECK (assessment_state != 'approved' OR (assessment_ref IS NOT NULL AND assessment_at IS NOT NULL))
);
CREATE INDEX idx_sources_retailer ON sources(retailer_id, source_key);

CREATE TABLE canonical_products (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 240),
    brand TEXT,
    product_line TEXT,
    variant TEXT,
    quantity_numerator INTEGER NOT NULL CHECK (quantity_numerator > 0),
    quantity_denominator INTEGER NOT NULL CHECK (quantity_denominator > 0),
    quantity_dimension TEXT NOT NULL CHECK (quantity_dimension IN ('volume','mass','count')),
    quantity_unit TEXT NOT NULL CHECK (quantity_unit IN ('L','kg','item')),
    pack_count INTEGER NOT NULL DEFAULT 1 CHECK (pack_count > 0),
    gtin TEXT CHECK (gtin IS NULL OR length(gtin) BETWEEN 8 AND 14),
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK ((quantity_dimension = 'volume' AND quantity_unit = 'L') OR
           (quantity_dimension = 'mass' AND quantity_unit = 'kg') OR
           (quantity_dimension = 'count' AND quantity_unit = 'item'))
);
CREATE INDEX idx_products_name ON canonical_products(archived, display_name, id);
CREATE INDEX idx_products_gtin ON canonical_products(gtin) WHERE gtin IS NOT NULL;

CREATE TABLE listings (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    canonical_product_id TEXT REFERENCES canonical_products(id) ON DELETE RESTRICT,
    external_sku TEXT,
    url TEXT NOT NULL CHECK (length(url) BETWEEN 1 AND 2048),
    url_key TEXT NOT NULL CHECK (length(url_key) BETWEEN 1 AND 2048),
    location_key TEXT NOT NULL DEFAULT '',
    match_state TEXT NOT NULL DEFAULT 'candidate'
        CHECK (match_state IN ('candidate','exact','possible','rejected','suspended')),
    match_reason TEXT,
    match_algorithm TEXT,
    first_seen_at INTEGER NOT NULL,
    last_checked_at INTEGER,
    created_at INTEGER NOT NULL,
    UNIQUE (source_id, url_key, location_key),
    UNIQUE (id, canonical_product_id),
    UNIQUE (id, source_id),
    CHECK (match_state != 'exact' OR canonical_product_id IS NOT NULL)
);
CREATE INDEX idx_listings_product_match ON listings(canonical_product_id, match_state, id);
CREATE INDEX idx_listings_source_state ON listings(source_id, match_state, id);

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    job_kind TEXT NOT NULL CHECK (job_kind IN ('refresh_listing','source_probe','notification_delivery','maintenance')),
    source_id TEXT REFERENCES sources(id) ON DELETE RESTRICT,
    listing_id TEXT REFERENCES listings(id) ON DELETE RESTRICT,
    dedupe_key TEXT NOT NULL UNIQUE CHECK (length(dedupe_key) BETWEEN 1 AND 200),
    status TEXT NOT NULL CHECK (status IN ('queued','leased','retry_at','completed','blocked','dead')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    due_at INTEGER NOT NULL,
    lease_until INTEGER,
    leased_by TEXT,
    last_error_code TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (listing_id, source_id) REFERENCES listings(id, source_id) ON DELETE RESTRICT,
    CHECK ((job_kind = 'refresh_listing' AND source_id IS NOT NULL AND listing_id IS NOT NULL) OR
           (job_kind = 'source_probe' AND source_id IS NOT NULL AND listing_id IS NULL) OR
           (job_kind IN ('notification_delivery','maintenance') AND source_id IS NULL AND listing_id IS NULL)),
    CHECK ((status = 'leased' AND lease_until IS NOT NULL AND leased_by IS NOT NULL) OR
           (status != 'leased' AND lease_until IS NULL AND leased_by IS NULL))
);
CREATE INDEX idx_jobs_due ON jobs(status, due_at, created_at, id);
CREATE INDEX idx_jobs_lease ON jobs(lease_until) WHERE status = 'leased';

CREATE TABLE observations (
    id TEXT PRIMARY KEY,
    listing_id TEXT NOT NULL REFERENCES listings(id) ON DELETE RESTRICT,
    job_id TEXT REFERENCES jobs(id) ON DELETE RESTRICT,
    observed_at INTEGER NOT NULL CHECK (observed_at > 0),
    fetch_status TEXT NOT NULL CHECK (fetch_status IN ('success','blocked','rate_limited','timeout','http_error','network_error','oversized','parse_error','resource_deferred')),
    http_status INTEGER CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    parser_version TEXT,
    evidence_digest TEXT CHECK (evidence_digest IS NULL OR length(evidence_digest) = 64),
    evidence_location TEXT CHECK (evidence_location IS NULL OR length(evidence_location) <= 512),
    observed_title TEXT CHECK (observed_title IS NULL OR length(observed_title) <= 240),
    observed_gtin TEXT CHECK (observed_gtin IS NULL OR length(observed_gtin) BETWEEN 8 AND 14),
    observed_quantity_json TEXT CHECK (observed_quantity_json IS NULL OR length(observed_quantity_json) <= 256),
    observed_stock TEXT NOT NULL DEFAULT 'unknown' CHECK (observed_stock IN ('in_stock','out_of_stock','unknown')),
    failure_code TEXT,
    source_url TEXT NOT NULL CHECK (length(source_url) BETWEEN 1 AND 2048),
    validation_state TEXT NOT NULL CHECK (validation_state IN ('valid','invalid','uncertain')),
    CHECK (fetch_status = 'success' OR validation_state != 'valid')
);
CREATE INDEX idx_observations_latest ON observations(listing_id, observed_at DESC, id DESC);
CREATE INDEX idx_observations_job ON observations(job_id) WHERE job_id IS NOT NULL;

CREATE TABLE observed_offers (
    id TEXT PRIMARY KEY,
    observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE RESTRICT,
    offer_kind TEXT NOT NULL CHECK (offer_kind IN ('standard','public_special','member','multibuy','coupon','unknown')),
    currency TEXT NOT NULL CHECK (length(currency) = 3 AND currency NOT GLOB '*[^A-Z]*'),
    currency_scale INTEGER NOT NULL DEFAULT 2 CHECK (currency_scale BETWEEN 0 AND 3),
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    original_claim_minor INTEGER CHECK (original_claim_minor IS NULL OR original_claim_minor > 0),
    minimum_qty INTEGER NOT NULL DEFAULT 1 CHECK (minimum_qty > 0),
    membership_label TEXT CHECK (membership_label IS NULL OR length(membership_label) <= 160),
    coupon_condition TEXT CHECK (coupon_condition IS NULL OR length(coupon_condition) <= 512),
    valid_from INTEGER,
    valid_until INTEGER,
    offer_evidence TEXT NOT NULL CHECK (length(offer_evidence) BETWEEN 1 AND 512),
    eligibility_state TEXT NOT NULL CHECK (eligibility_state IN ('public','conditional','unknown')),
    CHECK (offer_kind = 'multibuy' OR minimum_qty = 1),
    CHECK (offer_kind != 'coupon' OR eligibility_state != 'public'),
    CHECK (offer_kind != 'member' OR eligibility_state = 'conditional'),
    CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until >= valid_from)
);
CREATE INDEX idx_offers_observation ON observed_offers(observation_id, id);

CREATE TABLE watches (
    id TEXT PRIMARY KEY,
    canonical_product_id TEXT NOT NULL REFERENCES canonical_products(id) ON DELETE RESTRICT,
    listing_id TEXT,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
    created_at INTEGER NOT NULL,
    current_rule_version INTEGER NOT NULL DEFAULT 1 CHECK (current_rule_version > 0),
    FOREIGN KEY (listing_id, canonical_product_id) REFERENCES listings(id, canonical_product_id) ON DELETE RESTRICT
);
CREATE INDEX idx_watches_active ON watches(active, canonical_product_id, id);

CREATE TABLE watch_rule_versions (
    watch_id TEXT NOT NULL REFERENCES watches(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version > 0),
    target_minor INTEGER CHECK (target_minor > 0),
    drop_basis_points INTEGER CHECK (drop_basis_points BETWEEN 1 AND 9900),
    condition_operator TEXT NOT NULL DEFAULT 'any' CHECK (condition_operator IN ('any','all')),
    allow_member INTEGER NOT NULL DEFAULT 0 CHECK (allow_member IN (0,1)),
    allow_multibuy INTEGER NOT NULL DEFAULT 0 CHECK (allow_multibuy IN (0,1)),
    rearm_policy_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL,
    PRIMARY KEY (watch_id, version),
    CHECK (target_minor IS NOT NULL OR drop_basis_points IS NOT NULL),
    CHECK (condition_operator != 'all' OR (target_minor IS NOT NULL AND drop_basis_points IS NOT NULL))
);
CREATE TRIGGER watch_rule_version_matches_current
BEFORE UPDATE OF current_rule_version ON watches
WHEN NEW.current_rule_version <= OLD.current_rule_version
BEGIN
    SELECT RAISE(ABORT, 'watch rule version must increase');
END;

CREATE TABLE alert_episodes (
    id TEXT PRIMARY KEY,
    watch_id TEXT NOT NULL REFERENCES watches(id) ON DELETE RESTRICT,
    rule_version INTEGER NOT NULL,
    listing_id TEXT NOT NULL REFERENCES listings(id) ON DELETE RESTRICT,
    episode_number INTEGER NOT NULL CHECK (episode_number > 0),
    observed_offer_id TEXT NOT NULL REFERENCES observed_offers(id) ON DELETE RESTRICT,
    episode_key TEXT NOT NULL UNIQUE CHECK (length(episode_key) BETWEEN 1 AND 240),
    reason_code TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    closed_at INTEGER,
    FOREIGN KEY (watch_id, rule_version) REFERENCES watch_rule_versions(watch_id, version) ON DELETE RESTRICT,
    UNIQUE (watch_id, rule_version, listing_id, episode_number),
    CHECK (closed_at IS NULL OR closed_at >= created_at)
);
CREATE INDEX idx_episodes_watch_open ON alert_episodes(watch_id, created_at DESC) WHERE closed_at IS NULL;
CREATE UNIQUE INDEX idx_episodes_open_scope ON alert_episodes(watch_id, rule_version, listing_id) WHERE closed_at IS NULL;

CREATE TABLE delivery_outbox (
    id TEXT PRIMARY KEY,
    alert_episode_id TEXT NOT NULL REFERENCES alert_episodes(id) ON DELETE RESTRICT,
    destination_key TEXT NOT NULL CHECK (length(destination_key) BETWEEN 1 AND 120),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (length(idempotency_key) BETWEEN 1 AND 200),
    status TEXT NOT NULL CHECK (status IN ('pending','leased','retry_at','delivered','dead')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at INTEGER NOT NULL,
    lease_until INTEGER,
    leased_by TEXT,
    last_status_code INTEGER CHECK (last_status_code IS NULL OR last_status_code BETWEEN 100 AND 599),
    last_error_code TEXT,
    delivered_at INTEGER,
    UNIQUE (alert_episode_id, destination_key),
    CHECK ((status = 'leased' AND lease_until IS NOT NULL AND leased_by IS NOT NULL) OR
           (status != 'leased' AND lease_until IS NULL AND leased_by IS NULL)),
    CHECK ((status = 'delivered' AND delivered_at IS NOT NULL) OR
           (status != 'delivered' AND delivered_at IS NULL))
);
CREATE INDEX idx_outbox_due ON delivery_outbox(status, next_attempt_at, id);

CREATE TABLE admin_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(trim(username)) BETWEEN 1 AND 80),
    password_hash TEXT NOT NULL CHECK (length(password_hash) BETWEEN 20 AND 512),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY CHECK (length(token_hash) = 64),
    admin_user_id TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    csrf_hash TEXT NOT NULL CHECK (length(csrf_hash) = 64),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL CHECK (expires_at > created_at),
    last_seen_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE source_events (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    event_code TEXT NOT NULL,
    detail_redacted TEXT,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_source_events_recent ON source_events(source_id, created_at DESC, id DESC);

CREATE TABLE match_reviews (
    id TEXT PRIMARY KEY,
    listing_id TEXT NOT NULL REFERENCES listings(id) ON DELETE RESTRICT,
    admin_user_id TEXT REFERENCES admin_users(id) ON DELETE SET NULL,
    previous_state TEXT NOT NULL,
    new_state TEXT NOT NULL CHECK (new_state IN ('exact','possible','rejected','suspended')),
    rationale TEXT NOT NULL CHECK (length(trim(rationale)) BETWEEN 1 AND 1024),
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_match_reviews_listing ON match_reviews(listing_id, created_at DESC);

CREATE TABLE admin_audit (
    id TEXT PRIMARY KEY,
    admin_user_id TEXT REFERENCES admin_users(id) ON DELETE SET NULL,
    action_code TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT,
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_admin_audit_recent ON admin_audit(created_at DESC, id DESC);

CREATE TABLE app_settings (
    setting_key TEXT PRIMARY KEY CHECK (length(setting_key) BETWEEN 1 AND 120),
    setting_value TEXT NOT NULL CHECK (length(setting_value) <= 4096),
    updated_at INTEGER NOT NULL
);

-- Historical observations and their extracted offers are evidence. Correct a
-- parser or price by appending a new observation, never by rewriting history.
CREATE TRIGGER observations_no_update
BEFORE UPDATE ON observations
BEGIN
    SELECT RAISE(ABORT, 'observations are immutable');
END;
CREATE TRIGGER observations_no_delete
BEFORE DELETE ON observations
BEGIN
    SELECT RAISE(ABORT, 'observations are immutable');
END;
CREATE TRIGGER offers_no_update
BEFORE UPDATE ON observed_offers
BEGIN
    SELECT RAISE(ABORT, 'observed offers are immutable');
END;
CREATE TRIGGER offers_no_delete
BEFORE DELETE ON observed_offers
BEGIN
    SELECT RAISE(ABORT, 'observed offers are immutable');
END;
CREATE TRIGGER watch_rules_no_update
BEFORE UPDATE ON watch_rule_versions
BEGIN
    SELECT RAISE(ABORT, 'watch rule versions are immutable');
END;
CREATE TRIGGER watch_rules_no_delete
BEFORE DELETE ON watch_rule_versions
BEGIN
    SELECT RAISE(ABORT, 'watch rule versions are immutable');
END;
