-- The address of the person's picture at their sign-in provider, refreshed at each sign-in.
-- Only the address is kept, never the image.
ALTER TABLE profiles ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
