-- Allow a link_token to be scoped to an existing item for re-authentication.
-- When relink_item_id is set, the OAuth callback updates the existing item's
-- provider tokens instead of creating a new item.
ALTER TABLE link_tokens
    ADD COLUMN IF NOT EXISTS relink_item_id UUID REFERENCES items(id);
