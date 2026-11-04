-- Migration 011: idempotency key notifikasi jadi per penerima.
--
-- Script 010 memakai UNIQUE (outbox_id) karena satu event KRS hanya
-- menjangkau satu mahasiswa. Event course.quota_changed menjangkau SELURUH
-- peserta kelas: satu baris outbox menghasilkan banyak notifikasi, jadi
-- kuncinya digeser menjadi (outbox_id, user_id).
--
-- Kunci baru ini juga membuat "penerima yang belum diberi tahu" bisa
-- dicari: LEFT JOIN ke tabel notifications dan membandingkan jumlahnya
-- dengan jumlah peserta di payload event.
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_outbox_unique;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_outbox_user_unique UNIQUE (outbox_id, user_id);
