ALTER TABLE template_types ADD COLUMN sender_identity_id UUID REFERENCES adapter_identities(id) ON DELETE SET NULL;
