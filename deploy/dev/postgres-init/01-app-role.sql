-- Dev only. Creates a NON-superuser runtime role so Row-Level Security is
-- actually enforced — superusers (and the default POSTGRES_USER) bypass RLS.
--
-- The services connect as this role. It is granted CREATE on the schema, so the
-- tables it creates at migration time are owned by it; combined with
-- FORCE ROW LEVEL SECURITY (set by the migration), the org-isolation policy then
-- applies to the role itself. Runs once, on first database initialization.
CREATE ROLE mcp_app WITH LOGIN PASSWORD 'mcp_app'
  NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;

GRANT CONNECT ON DATABASE mcp TO mcp_app;
GRANT USAGE, CREATE ON SCHEMA public TO mcp_app;
