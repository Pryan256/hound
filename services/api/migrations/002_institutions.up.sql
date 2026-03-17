-- Trigram extension for fuzzy name search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE institutions (
    id              TEXT PRIMARY KEY,       -- e.g. "chase", "bofa" — stable, URL-safe
    name            TEXT NOT NULL,
    logo_url        TEXT,
    primary_color   TEXT,                  -- hex, e.g. "#117ACA" — for Link UI branding
    url             TEXT,                  -- institution's website
    provider        TEXT NOT NULL CHECK (provider IN ('akoya', 'finicity', 'sandbox')),
    products        TEXT[] NOT NULL DEFAULT '{"transactions","identity"}',
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'down')),
    oauth_only      BOOLEAN NOT NULL DEFAULT false,  -- true = OAuth flow only (no credentials)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GIN index on trigram for fast ILIKE / similarity search
CREATE INDEX idx_institutions_name_trgm ON institutions USING gin (name gin_trgm_ops);

-- Full-text search vector for "bank of america" → "bank america" style matching
ALTER TABLE institutions ADD COLUMN name_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('english', name)) STORED;

CREATE INDEX idx_institutions_name_tsv ON institutions USING gin (name_tsv);

-- -----------------------------------------------------------------------
-- Seed: Major US institutions
-- Logo URLs use Clearbit Logo API (public, no auth required)
-- -----------------------------------------------------------------------
INSERT INTO institutions (id, name, logo_url, primary_color, url, provider, oauth_only) VALUES
-- Akoya network (OAuth-only, major banks)
('chase',           'Chase',                         'https://logo.clearbit.com/chase.com',          '#117ACA', 'https://www.chase.com',          'akoya',    true),
('bofa',            'Bank of America',               'https://logo.clearbit.com/bankofamerica.com',  '#E31837', 'https://www.bankofamerica.com',   'akoya',    true),
('wellsfargo',      'Wells Fargo',                   'https://logo.clearbit.com/wellsfargo.com',     '#D71E28', 'https://www.wellsfargo.com',      'akoya',    true),
('capitalonebank',  'Capital One',                   'https://logo.clearbit.com/capitalone.com',     '#D03027', 'https://www.capitalone.com',      'akoya',    true),
('usbank',          'U.S. Bank',                     'https://logo.clearbit.com/usbank.com',         '#003087', 'https://www.usbank.com',          'akoya',    true),
('citibank',        'Citi',                          'https://logo.clearbit.com/citi.com',           '#003B70', 'https://www.citibank.com',        'akoya',    true),
('pnc',             'PNC Bank',                      'https://logo.clearbit.com/pnc.com',            '#F58025', 'https://www.pnc.com',             'akoya',    true),
('tdbank',          'TD Bank',                       'https://logo.clearbit.com/td.com',             '#2C6B37', 'https://www.td.com',              'akoya',    true),
('truist',          'Truist',                        'https://logo.clearbit.com/truist.com',         '#4E2280', 'https://www.truist.com',          'akoya',    true),
('regions',         'Regions Bank',                  'https://logo.clearbit.com/regions.com',        '#005587', 'https://www.regions.com',         'akoya',    true),

-- Finicity fallback (credential-based or Finicity OAuth)
('ally',            'Ally Bank',                     'https://logo.clearbit.com/ally.com',           '#6A0DAD', 'https://www.ally.com',            'finicity', false),
('schwab',          'Charles Schwab',                'https://logo.clearbit.com/schwab.com',         '#00A0DF', 'https://www.schwab.com',          'finicity', false),
('fidelity',        'Fidelity',                      'https://logo.clearbit.com/fidelity.com',       '#008A00', 'https://www.fidelity.com',        'finicity', false),
('vanguard',        'Vanguard',                      'https://logo.clearbit.com/vanguard.com',       '#8B0000', 'https://investor.vanguard.com',   'finicity', false),
('navyfederal',     'Navy Federal Credit Union',     'https://logo.clearbit.com/navyfederal.org',    '#003087', 'https://www.navyfederal.org',     'finicity', false),
('usaa',            'USAA',                          'https://logo.clearbit.com/usaa.com',           '#003087', 'https://www.usaa.com',            'finicity', false),
('discover',        'Discover',                      'https://logo.clearbit.com/discover.com',       '#F76F20', 'https://www.discover.com',        'finicity', false),
('americanexpress', 'American Express',              'https://logo.clearbit.com/americanexpress.com','#007BC1', 'https://www.americanexpress.com', 'finicity', false),
('synchrony',       'Synchrony Bank',                'https://logo.clearbit.com/synchrony.com',      '#004B87', 'https://www.synchrony.com',       'finicity', false),
('sofi',            'SoFi',                          'https://logo.clearbit.com/sofi.com',           '#00C896', 'https://www.sofi.com',            'finicity', false),
('chime',           'Chime',                         'https://logo.clearbit.com/chime.com',          '#1EC677', 'https://www.chime.com',           'finicity', false),
('marcus',          'Marcus by Goldman Sachs',       'https://logo.clearbit.com/marcus.com',         '#374151', 'https://www.marcus.com',          'finicity', false),
('citizensbank',    'Citizens Bank',                 'https://logo.clearbit.com/citizensbank.com',   '#00833E', 'https://www.citizensbank.com',    'finicity', false),
('fifththird',      'Fifth Third Bank',              'https://logo.clearbit.com/53.com',             '#006B3F', 'https://www.53.com',              'finicity', false),
('keybank',         'KeyBank',                       'https://logo.clearbit.com/key.com',            '#CC0000', 'https://www.key.com',             'finicity', false),
('huntington',      'Huntington Bank',               'https://logo.clearbit.com/huntington.com',     '#00AA4F', 'https://www.huntington.com',      'finicity', false),
('bbt',             'Truist (BB&T)',                 'https://logo.clearbit.com/bbt.com',            '#4E2280', 'https://www.bbt.com',             'finicity', false),
('suntrustbank',    'Truist (SunTrust)',              'https://logo.clearbit.com/suntrust.com',       '#4E2280', 'https://www.suntrust.com',        'finicity', false),
('morganstanley',   'Morgan Stanley',                'https://logo.clearbit.com/morganstanley.com',  '#003087', 'https://www.morganstanley.com',   'finicity', false),
('merrilledge',     'Merrill Edge',                  'https://logo.clearbit.com/merrilledge.com',    '#E31837', 'https://www.merrilledge.com',     'finicity', false),

-- Sandbox (only available in test environment)
('ins_sandbox',     'Sandbox Bank',                  NULL, '#6366F1', NULL, 'sandbox', false);
