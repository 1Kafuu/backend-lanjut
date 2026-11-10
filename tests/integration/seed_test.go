//go:build integration

package integration_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"api-students/app/model"
)

func uniqueSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("acak: %v", err)
	}
	return hex.EncodeToString(b)
}

func seedStudent(t *testing.T) model.User {
	t.Helper()
	ctx := context.Background()
	s := uniqueSuffix(t)

	u := model.User{Username: "it_" + s, Email: "it_" + s + "@student.unair.ac.id", Role: "user"}
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password, role) VALUES ($1, $2, '!', 'user') RETURNING id`,
		u.Username, u.Email).Scan(&u.ID)
	if err != nil {
		t.Fatalf("seed mahasiswa: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM outbox WHERE (payload->>'user_id')::INT = $1`, u.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u
}

func seedCourse(t *testing.T, credits, quota int) model.Course {
	t.Helper()
	ctx := context.Background()

	c := model.Course{Code: "IT" + uniqueSuffix(t), Name: "Kelas Uji", Credits: credits, Quota: quota}
	err := pool.QueryRow(ctx,
		`INSERT INTO courses (code, name, credits, quota) VALUES ($1, $2, $3, $4) RETURNING id`,
		c.Code, c.Name, c.Credits, c.Quota).Scan(&c.ID)
	if err != nil {
		t.Fatalf("seed kelas: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM courses WHERE id = $1`, c.ID)
	})
	return c
}
