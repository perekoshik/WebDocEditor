drop index if exists documents_is_template_idx;
alter table documents drop column if exists is_template;
