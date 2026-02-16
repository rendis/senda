CREATE TABLE template_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID NOT NULL REFERENCES templates(id),
    version_number  INT NOT NULL,
    status          version_status NOT NULL DEFAULT 'draft',
    subject         TEXT NOT NULL DEFAULT '',
    preview_text    TEXT NOT NULL DEFAULT '',
    from_name       TEXT NOT NULL DEFAULT '',
    from_email      VARCHAR(255) NOT NULL DEFAULT '',
    reply_to        VARCHAR(255),
    body_mjml       TEXT NOT NULL DEFAULT '',
    default_locale  VARCHAR(10) NOT NULL DEFAULT 'en',
    editor_data     JSONB,
    created_by      UUID REFERENCES members(id),
    published_at    TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tv_version_unique UNIQUE (template_id, version_number),
    CONSTRAINT tv_one_published
        EXCLUDE USING btree (template_id WITH =) WHERE (status = 'published')
);

CREATE INDEX idx_tv_template_status ON template_versions (template_id, status);
CREATE INDEX idx_tv_published ON template_versions (template_id) WHERE status = 'published';

CREATE TABLE template_version_locales (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_version_id     UUID NOT NULL REFERENCES template_versions(id) ON DELETE CASCADE,
    locale                  VARCHAR(10) NOT NULL,
    subject                 TEXT,
    preview_text            TEXT,
    from_name               TEXT,
    body_mjml               TEXT,
    editor_data             JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tvl_unique UNIQUE (template_version_id, locale)
);
