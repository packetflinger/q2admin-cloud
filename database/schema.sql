CREATE TABLE IF NOT EXISTS "system_log" (
	"id"	INTEGER,
	"log_time"	INTEGER,
	"log_entry"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "client_log" (
	"id"	INTEGER,
	"uuid"	TEXT,
	"event_time"	INTEGER,
	"event"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "chat" (
	"id"	INTEGER,
	"uuid"	TEXT,
	"chat_time"	INTEGER,
	"chat"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "player" (
	"id"	INTEGER,
	"server"	TEXT,
	"name"	TEXT,
	"ip"	TEXT,
	"hash"	TEXT,
	"userinfo"	TEXT,
	"connect_time"	INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);
