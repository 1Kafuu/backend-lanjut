-- seed12.sql — fixture uji notifikasi kuota (D.2)
--   psql -U postgres -d praktikum_backend -f seed12.sql
--
-- 1 admin + 8 peserta pada satu kelas uji. Password '!' tidak bisa dipakai
-- login: token dibuat sendiri oleh tools/loadtest -mode token.

INSERT INTO users (username, email, password, role) VALUES
    ('d2admin', 'd2admin@kampus.ac.id', '!', 'admin')
ON CONFLICT DO NOTHING;

INSERT INTO users (username, email, password, role)
SELECT 'd2p' || g::TEXT,
       'd2p' || g::TEXT || '@student.unair.ac.id',
       '!', 'user'
FROM generate_series(1, 8) AS g
ON CONFLICT DO NOTHING;

INSERT INTO courses (code, name, credits, quota) VALUES
    ('IF397', 'Kelas Uji Kuota', 3, 40)
ON CONFLICT (code) DO NOTHING;

INSERT INTO enrollments (course_id, user_id)
SELECT c.id, u.id
FROM courses c CROSS JOIN users u
WHERE c.code = 'IF397' AND u.username LIKE 'd2p%'
ON CONFLICT DO NOTHING;

UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id)
WHERE c.code = 'IF397';
