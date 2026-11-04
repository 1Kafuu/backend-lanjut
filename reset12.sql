-- reset12.sql — mengembalikan keadaan setelah uji notifikasi kuota (D.2)
--   psql -U postgres -d praktikum_backend -f reset12.sql
--
-- HANYA menyentuh akun uji d2admin dan d2p1..d2p8, kelas uji IF397,
-- dan event yang dipakai akun uji tersebut.

-- 1. Notifikasi dan event milik akun uji.
DELETE FROM notifications
WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'd2p%');

DELETE FROM outbox
WHERE event_type = 'course.quota_changed'
   OR (payload->>'user_id')::INT IN (SELECT id FROM users WHERE username LIKE 'd2p%');

-- 2. KRS dan akun uji.
DELETE FROM enrollments
WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'd2p%');

DELETE FROM users WHERE username LIKE 'd2p%' OR username = 'd2admin';

-- 3. Kelas uji.
DELETE FROM courses WHERE code = 'IF397';

-- 4. Counter peserta dihitung ulang dari baris enrollments yang sebenarnya.
UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);
