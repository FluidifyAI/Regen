-- Drop grouping_rules table and all associated indexes and constraints
--
-- WARNING: This migration will delete all grouping rules configuration.
-- Existing incidents are NOT affected (they retain their alert associations).

DROP TABLE IF EXISTS grouping_rules CASCADE;
