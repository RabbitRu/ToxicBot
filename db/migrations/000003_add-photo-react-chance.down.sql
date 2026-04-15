CREATE TABLE chat_settings_backup AS SELECT
    chat_id
    ,threshold_count
    ,threshold_time_ns
    ,cooldown_ns
    ,sticker_react_chance
    ,voice_react_chance
    ,ai_chance
    ,updated_at
FROM chat_settings;

DROP TABLE chat_settings;

CREATE TABLE chat_settings (
    chat_id              INTEGER PRIMARY KEY
    ,threshold_count     INTEGER
    ,threshold_time_ns   INTEGER
    ,cooldown_ns         INTEGER
    ,sticker_react_chance REAL
    ,voice_react_chance   REAL
    ,ai_chance            REAL
    ,updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO chat_settings SELECT * FROM chat_settings_backup;
DROP TABLE chat_settings_backup;
