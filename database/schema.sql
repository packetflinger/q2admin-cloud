CREATE TABLE IF NOT EXISTS "chat" (
	"id"	INTEGER,
	"uuid"	TEXT,
	"chat_time"	INTEGER,
	"chat"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "connection" (
	"id"	INTEGER,
	"frontend"	INTEGER NOT NULL DEFAULT 0 UNIQUE,
	"last_seen"	INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "frontend" (
	"id"	INTEGER,
	"uuid"	TEXT NOT NULL DEFAULT "",
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "player" (
	"id"	INTEGER,
	"server_id"	INTEGER,
	"name"	TEXT,
	"ip"	TEXT,
	"hostname"	TEXT,
	"vpn"	INTEGER,
	"cookie"	TEXT,
	"version"	TEXT,
	"userinfo"	TEXT,
	"time"	INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);

