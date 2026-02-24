-- Add teams_conversation_id to store the Bot Framework conversation ID per incident.
-- This is required for Bot Framework Proactive Messaging — the conversation ID
-- is returned when a new conversation is created in a channel via the Bot Framework
-- REST API, and is needed for subsequent PostToConversation and UpdateConversationMessage calls.
-- It is distinct from the Teams channel ID (19:xxx@thread.tacv2) stored in teams_channel_id.

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS teams_conversation_id VARCHAR(500);

COMMENT ON COLUMN incidents.teams_conversation_id IS 'Bot Framework conversation ID (a:xxx) for the incident channel — used for proactive messaging';
