-- Persist the user-visible artwork name for image workbench jobs. The same
-- durable job record powers both the task queue and completed artwork library.

ALTER TABLE image_generation_jobs
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(80);

COMMENT ON COLUMN image_generation_jobs.display_name IS
    'Optional user-defined artwork name shown by the image workbench';
