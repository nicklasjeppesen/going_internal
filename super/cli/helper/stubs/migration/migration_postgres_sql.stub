
-- -------------------------------------------------------------

-- This script only contains the table creation statements and does not fully represent the table in database. It's still missing: indices, triggers. Do not use it as backup.

-- Sequences
CREATE SEQUENCE IF NOT EXISTS untitled_table_id_seq;

-- Table Definition
CREATE TABLE "public"."users" (
    "id" int4 NOT NULL DEFAULT nextval('untitled_table_id_seq'::regclass),
    "name" varchar,
    "age" int8,
    "email" varchar,
    "password" varchar, 
    "sessiontoken" varchar,
    "csrftoken" varchar,
    "created_at" timestamp NOT NULL DEFAULT now(),
    "updated_at" timestamp NOT NULL DEFAULT now(),
    
    PRIMARY KEY ("id")
);
