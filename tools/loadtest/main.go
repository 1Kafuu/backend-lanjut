// Alat uji beban Modul 9. Bukan bagian aplikasi dan tidak pernah diimpor
// main.go. Jalankan dari root proyek:
//
//	go run ./tools/loadtest -mode crowd
//	go run ./tools/loadtest -mode credits
//	go run ./tools/loadtest -mode swap
//	go run ./tools/loadtest -mode token -user mhs001
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"api-students/app/model"
	"api-students/config"
	"api-students/database"
	"api-students/helper"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultBase = "http://localhost:3000/api/v1"
	crowdCourse = "IF399"
	swapCourseA = "IF101"
	swapCourseB = "IF102"
	maxCredits  = 24
)

// mhs menyimpan data akun uji beserta token aksesnya.
type mhs struct {
	id       int
	username string
	role     string
	token    string
}

// harness menyimpan koneksi database, client HTTP, dan konfigurasi uji.
type harness struct {
	pool  *pgxpool.Pool
	http  *http.Client
	base  string
	jwt   *helper.JWTManager
	reset bool
}

func main() {
	mode := flag.String("mode", "", "crowd | credits | swap | token")
	user := flag.String("user", "mhs001", "username untuk -mode token")
	base := flag.String("base", defaultBase, "base URL API")
	resetFirst := flag.Bool("reset", true, "bersihkan enrollment akun seed mhs001 sampai mhs060 sebelum menjalankan mode")
	flag.Parse()

	config.LoadENV()

	if *mode == "" {
		fmt.Println("pilih -mode crowd|credits|swap|token")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Println("gagal terhubung ke database:", err)
		os.Exit(1)
	}
	defer pool.Close()

	secret := config.GetENV("JWT_SECRET", "")
	if len(secret) < 32 {
		fmt.Println("JWT_SECRET tidak diisi atau terlalu pendek")
		os.Exit(1)
	}
	jwtManager := helper.NewJWTManager(
		secret,
		config.GetENV("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetENVInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)

	h := &harness{
		pool:  pool,
		http:  &http.Client{Timeout: 15 * time.Second},
		base:  *base,
		jwt:   jwtManager,
		reset: *resetFirst,
	}

	switch *mode {
	case "token":
		h.printToken(ctx, *user)
	case "crowd":
		h.runCrowd(ctx)
	case "credits":
		h.runCredits(ctx)
	case "swap":
		h.runSwap(ctx)
	default:
		fmt.Println("mode tidak dikenal:", *mode)
		os.Exit(1)
	}
}

// fire menjalankan job sebanyak n kali secara serentak.
// Gerbang close(start) membuat semua goroutine berangkat bersamaan,
// bukan bergiliran seperti loop biasa.
func fire(n int, job func(i int) string) map[string]int {
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- job(i)
		}(i)
	}

	close(start)
	wg.Wait()
	close(results)

	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

// printTally mencetak jumlah hasil per label dengan urutan stabil.
func printTally(tally map[string]int) {
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%-30s %d\n", k, tally[k])
	}
}

// postJSON mengirim request POST dan mengembalikan label hasil,
// misalnya "201 OK" atau "409 COURSE_FULL".
func (h *harness) postJSON(ctx context.Context, path, token string, body any) string {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return "0 marshal_error"
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.base+path, reader)
	if err != nil {
		return "0 request_error"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := h.http.Do(req)
	if err != nil {
		return "client timeout"
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 400 {
		return fmt.Sprintf("%d OK", resp.StatusCode)
	}

	var errResp model.ErrorResponse
	if err := json.Unmarshal(raw, &errResp); err == nil && errResp.Code != "" {
		return fmt.Sprintf("%d %s", resp.StatusCode, errResp.Code)
	}
	return fmt.Sprintf("%d", resp.StatusCode)
}

// mustReset menghapus enrollment akun uji lalu menghitung ulang enrolled_count.
func (h *harness) mustReset(ctx context.Context) {
	if _, err := h.pool.Exec(ctx,
		`DELETE FROM enrollments
		 WHERE user_id IN (SELECT id FROM users WHERE username ~ '^mhs[0-9]{3}$')`); err != nil {
		fmt.Println("gagal membersihkan enrollments:", err)
		os.Exit(1)
	}
	h.mustRecomputeCounts(ctx)
}

// mustRecomputeCounts menyamakan enrolled_count dengan jumlah baris enrollments.
func (h *harness) mustRecomputeCounts(ctx context.Context) {
	if _, err := h.pool.Exec(ctx,
		`UPDATE courses c
		 SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id)`); err != nil {
		fmt.Println("gagal menghitung ulang enrolled_count:", err)
		os.Exit(1)
	}
}

// prepare membersihkan sisa data uji. Gunakan -reset=false untuk uji C.5
// yang sengaja tidak melakukan reset di antara percobaan.
func (h *harness) prepare(ctx context.Context) {
	if h.reset {
		h.mustReset(ctx)
	}
}

// mustLoadMhs memuat akun seed mhs001 sampai mhs060 beserta token aksesnya.
func (h *harness) mustLoadMhs(ctx context.Context) []mhs {
	rows, err := h.pool.Query(ctx,
		"SELECT id, username, role FROM users WHERE username ~ '^mhs[0-9]{3}$' ORDER BY username")
	if err != nil {
		fmt.Println("gagal membaca users:", err)
		os.Exit(1)
	}
	defer rows.Close()

	result := []mhs{}
	for rows.Next() {
		var u mhs
		if err := rows.Scan(&u.id, &u.username, &u.role); err != nil {
			fmt.Println("gagal membaca row user:", err)
			os.Exit(1)
		}
		token, err := h.jwt.GenerateAccess(model.User{
			ID: u.id, Username: u.username, Role: u.role,
		})
		if err != nil {
			fmt.Println("gagal membuat token:", err)
			os.Exit(1)
		}
		u.token = token
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		fmt.Println("gagal membaca users:", err)
		os.Exit(1)
	}
	return result
}

// courseIDs memetakan kode kelas ke id.
func (h *harness) courseIDs(ctx context.Context) map[string]int {
	rows, err := h.pool.Query(ctx, "SELECT id, code FROM courses")
	if err != nil {
		fmt.Println("gagal membaca courses:", err)
		os.Exit(1)
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var id int
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			fmt.Println("gagal membaca row course:", err)
			os.Exit(1)
		}
		result[code] = id
	}
	return result
}

// printCourseConsistency membandingkan enrolled_count dengan jumlah baris enrollments.
func (h *harness) printCourseConsistency(ctx context.Context, code string, courseID int) {
	var quota, enrolled int
	if err := h.pool.QueryRow(ctx,
		"SELECT quota, enrolled_count FROM courses WHERE id = $1", courseID).
		Scan(&quota, &enrolled); err != nil {
		fmt.Println("gagal membaca kelas:", err)
		return
	}

	var rows int
	if err := h.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM enrollments WHERE course_id = $1", courseID).
		Scan(&rows); err != nil {
		fmt.Println("gagal menghitung enrollments:", err)
		return
	}

	status := "KONSISTEN"
	if enrolled != rows {
		status = "TIDAK KONSISTEN"
	}
	fmt.Printf("%s  kuota=%d  enrolled_count=%d  baris enrollments=%d  -> %s\n",
		code, quota, enrolled, rows, status)
}

// printCreditConsistency memeriksa apakah ada mahasiswa yang melebihi batas SKS.
func (h *harness) printCreditConsistency(ctx context.Context, users []mhs) {
	violations := []string{}
	for _, u := range users {
		var total int
		if err := h.pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(c.credits), 0)
			 FROM enrollments e
			 JOIN courses c ON c.id = e.course_id
			 WHERE e.user_id = $1`, u.id).Scan(&total); err != nil {
			fmt.Println("gagal menghitung SKS:", err)
			return
		}
		if total > maxCredits {
			violations = append(violations, fmt.Sprintf("%s=%d SKS", u.username, total))
		}
	}
	if len(violations) == 0 {
		fmt.Printf("seluruh %d mahasiswa <= %d SKS -> KONSISTEN\n", len(users), maxCredits)
		return
	}
	fmt.Printf("melebihi %d SKS: %v -> TIDAK KONSISTEN\n", maxCredits, violations)
}

// mustEnrollDirect mengisi enrollment langsung di database, tanpa lewat API.
func (h *harness) mustEnrollDirect(ctx context.Context, courseID int, users []mhs) {
	for _, u := range users {
		if _, err := h.pool.Exec(ctx,
			"INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			courseID, u.id); err != nil {
			fmt.Println("gagal menyiapkan enrollments:", err)
			os.Exit(1)
		}
	}
}

// deadlocks mengembalikan jumlah deadlock PostgreSQL sejak database dinyalakan.
func (h *harness) deadlocks(ctx context.Context) int {
	var n int
	if err := h.pool.QueryRow(ctx,
		"SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").
		Scan(&n); err != nil {
		fmt.Println("gagal membaca pg_stat_database:", err)
		os.Exit(1)
	}
	return n
}

// runCrowd membuat semua mahasiswa berebut satu kelas dengan kuota terbatas.
func (h *harness) runCrowd(ctx context.Context) {
	h.prepare(ctx)
	users := h.mustLoadMhs(ctx)
	ids := h.courseIDs(ctx)
	courseID, ok := ids[crowdCourse]
	if !ok {
		fmt.Println("kelas", crowdCourse, "tidak ditemukan; jalankan migrasi 007 dulu")
		os.Exit(1)
	}

	fmt.Printf("%d mahasiswa berebut kelas %s secara serentak...\n", len(users), crowdCourse)
	start := time.Now()
	tally := fire(len(users), func(i int) string {
		return h.postJSON(ctx, fmt.Sprintf("/courses/%d/enroll", courseID), users[i].token, nil)
	})
	fmt.Println("hasil request:")
	printTally(tally)
	fmt.Printf("waktu total : %s\n", time.Since(start).Round(time.Millisecond))
	h.printCourseConsistency(ctx, crowdCourse, courseID)
}

// runCredits membuat semua mahasiswa mendaftar ke banyak kelas sekaligus.
func (h *harness) runCredits(ctx context.Context) {
	h.prepare(ctx)
	users := h.mustLoadMhs(ctx)
	if len(users) < 10 {
		fmt.Println("butuh minimal 10 akun mhs; jalankan migrasi 008 dulu")
		os.Exit(1)
	}
	users = users[:10]

	ids := h.courseIDs(ctx)
	codes := []string{}
	for code := range ids {
		if code != crowdCourse {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	if len(codes) > 12 {
		codes = codes[:12]
	}

	fmt.Printf("%d mahasiswa, masing-masing mendaftar ke %d kelas 3 SKS secara serentak (batas %d SKS)...\n",
		len(users), len(codes), maxCredits)
	start := time.Now()
	tally := fire(len(users)*len(codes), func(i int) string {
		u := users[i/len(codes)]
		code := codes[i%len(codes)]
		return h.postJSON(ctx, fmt.Sprintf("/courses/%d/enroll", ids[code]), u.token, nil)
	})
	fmt.Println("hasil request:")
	printTally(tally)
	fmt.Printf("waktu total : %s\n", time.Since(start).Round(time.Millisecond))
	h.printCreditConsistency(ctx, users)
}

// runSwap membuat pasangan mahasiswa berpindah kelas berlawanan arah.
func (h *harness) runSwap(ctx context.Context) {
	h.prepare(ctx)
	users := h.mustLoadMhs(ctx)
	if len(users) < 40 {
		fmt.Println("butuh minimal 40 akun mhs; jalankan migrasi 008 dulu")
		os.Exit(1)
	}
	users = users[:40]

	ids := h.courseIDs(ctx)
	a, okA := ids[swapCourseA]
	b, okB := ids[swapCourseB]
	if !okA || !okB {
		fmt.Println("kelas", swapCourseA, "atau", swapCourseB, "tidak ditemukan; jalankan migrasi 007 dulu")
		os.Exit(1)
	}

	// Keadaan awal: mhs001 sampai mhs020 di IF101, mhs021 sampai mhs040 di IF102.
	h.mustEnrollDirect(ctx, a, users[:20])
	h.mustEnrollDirect(ctx, b, users[20:40])
	h.mustRecomputeCounts(ctx)

	before := h.deadlocks(ctx)
	fmt.Printf("%d pasang mahasiswa pindah kelas berlawanan arah secara serentak...\n", 20)
	start := time.Now()
	tally := fire(40, func(i int) string {
		u := users[i]
		from, to := a, b
		if i >= 20 {
			from, to = b, a
		}
		return h.postJSON(ctx, "/enrollments/swap", u.token, map[string]int{
			"from_course_id": from,
			"to_course_id":   to,
		})
	})
	fmt.Println("hasil request:")
	printTally(tally)
	fmt.Printf("waktu total : %s\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("deadlock terdeteksi PostgreSQL selama uji: %d\n", h.deadlocks(ctx)-before)
	h.printCourseConsistency(ctx, swapCourseA, a)
	h.printCourseConsistency(ctx, swapCourseB, b)
}

// printToken mencetak token akses untuk satu akun.
func (h *harness) printToken(ctx context.Context, username string) {
	var u mhs
	if err := h.pool.QueryRow(ctx,
		"SELECT id, username, role FROM users WHERE username = $1", username).
		Scan(&u.id, &u.username, &u.role); err != nil {
		fmt.Println("gagal membaca user", username, ":", err)
		os.Exit(1)
	}
	token, err := h.jwt.GenerateAccess(model.User{
		ID: u.id, Username: u.username, Role: u.role,
	})
	if err != nil {
		fmt.Println("gagal membuat token:", err)
		os.Exit(1)
	}
	fmt.Println(token)
}
