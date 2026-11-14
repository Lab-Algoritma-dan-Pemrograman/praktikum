-- Menghapus salinan baris legacy yang menumpuk di audit_logs.detail.row dan
-- academic_operations.data.row. Isi asli tetap utuh di legacy_preserve.rows,
-- dan kolom referensi (source_table/source_pk, legacy_table, kind) tetap ada,
-- sehingga tidak ada informasi yang hilang. Aman dijalankan berulang.
update public.audit_logs
set detail = detail - 'row'
where operation_id like 'legacy:%' and detail ? 'row';

update public.academic_operations
set data = data - 'row'
where data ? 'row';
