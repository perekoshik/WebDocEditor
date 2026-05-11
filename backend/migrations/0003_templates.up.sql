alter table documents
    add column is_template boolean not null default false;

create index documents_is_template_idx on documents (is_template);
