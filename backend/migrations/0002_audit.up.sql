create table audit_log (
    id          uuid primary key default gen_random_uuid(),
    action      text not null,
    document_id uuid references documents(id) on delete set null,
    metadata    jsonb not null default '{}'::jsonb,
    created_at  timestamptz not null default now()
);

create index audit_log_created_at_idx on audit_log (created_at desc);
create index audit_log_document_id_idx on audit_log (document_id);
create index audit_log_action_idx on audit_log (action);
