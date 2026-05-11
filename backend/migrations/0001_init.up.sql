create extension if not exists "pgcrypto";

create table documents (
    id           uuid primary key default gen_random_uuid(),
    title        text not null,
    filename     text not null,
    storage_path text not null,
    version      int  not null default 1,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now()
);

create index documents_updated_at_idx on documents (updated_at desc);
create index documents_title_idx on documents using gin (to_tsvector('simple', title));

create table document_versions (
    id           uuid primary key default gen_random_uuid(),
    document_id  uuid not null references documents(id) on delete cascade,
    version      int  not null,
    storage_path text not null,
    created_at   timestamptz not null default now(),
    unique (document_id, version)
);
