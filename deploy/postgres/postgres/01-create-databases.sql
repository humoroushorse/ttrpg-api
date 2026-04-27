-- Create application databases
-- This runs automatically on first postgres container start
-- (files in docker-entrypoint-initdb.d are executed in alphabetical order)

CREATE DATABASE auth;
CREATE DATABASE sprint_management;
CREATE DATABASE dnd;
