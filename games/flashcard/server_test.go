package flashcard

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, *store) {
	t.Helper()
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(newServer(st, 0).http.Handler)
	t.Cleanup(ts.Close)
	return ts, st
}

func uploadCard(t *testing.T, base, name string, includeImage, includeName bool) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if includeImage {
		fw, err := mw.CreateFormFile("image", "card.png")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(makePNG(t, 16, 16))
	}
	if includeName {
		mw.WriteField("name", name)
	}
	mw.Close()

	req, _ := http.NewRequest("POST", base+"/api/cards", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func getList(t *testing.T, base string) (cards []cardDTO, currentID string) {
	t.Helper()
	res, err := http.Get(base + "/api/cards")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Cards     []cardDTO `json:"cards"`
		CurrentID string    `json:"currentId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Cards, out.CurrentID
}

func doJSON(t *testing.T, method, url string, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestServerIndex(t *testing.T) {
	ts, _ := newTestServer(t)
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("index status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("index content-type %q", ct)
	}
	b, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(b), "Flashcards") {
		t.Fatal("index missing expected content")
	}
}

func TestServerUploadAndLifecycle(t *testing.T) {
	ts, _ := newTestServer(t)

	// Upload first card.
	res := uploadCard(t, ts.URL, "Apple", true, true)
	if res.StatusCode != 200 {
		t.Fatalf("upload status %d", res.StatusCode)
	}
	var created cardDTO
	json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	if created.ID == "" || created.Name != "Apple" {
		t.Fatalf("bad created card: %+v", created)
	}

	// It should be listed and auto-selected as current.
	cards, current := getList(t, ts.URL)
	if len(cards) != 1 || current != created.ID || !cards[0].IsCurrent {
		t.Fatalf("after first upload: cards=%+v current=%s", cards, current)
	}

	// A fresh card has exactly one photo; its image should be served as PNG.
	if len(created.Photos) != 1 {
		t.Fatalf("new card should have 1 photo, got %d", len(created.Photos))
	}
	imgRes, err := http.Get(ts.URL + "/api/cards/" + created.ID + "/photos/" + created.Photos[0].ID + "/image")
	if err != nil {
		t.Fatal(err)
	}
	if imgRes.StatusCode != 200 {
		t.Fatalf("image status %d", imgRes.StatusCode)
	}
	if _, err := png.Decode(imgRes.Body); err != nil {
		t.Fatalf("served image did not decode: %v", err)
	}
	imgRes.Body.Close()

	// Upload a second card and make it current.
	res = uploadCard(t, ts.URL, "Banana", true, true)
	var second cardDTO
	json.NewDecoder(res.Body).Decode(&second)
	res.Body.Close()

	if r := doJSON(t, "POST", ts.URL+"/api/current", `{"id":"`+second.ID+`"}`); r.StatusCode != 204 {
		t.Fatalf("set current status %d", r.StatusCode)
	}
	if _, current := getList(t, ts.URL); current != second.ID {
		t.Fatalf("current not updated: %s", current)
	}

	// next wraps back to the first card.
	doJSON(t, "POST", ts.URL+"/api/next", "")
	if _, current := getList(t, ts.URL); current != created.ID {
		t.Fatalf("next did not wrap to first: %s", current)
	}

	// Rename + reorder via PATCH.
	if r := doJSON(t, "PATCH", ts.URL+"/api/cards/"+created.ID, `{"name":"Avocado"}`); r.StatusCode != 204 {
		t.Fatalf("rename status %d", r.StatusCode)
	}
	if r := doJSON(t, "PATCH", ts.URL+"/api/cards/"+created.ID, `{"order":1}`); r.StatusCode != 204 {
		t.Fatalf("reorder status %d", r.StatusCode)
	}
	cards, _ = getList(t, ts.URL)
	if cards[1].ID != created.ID || cards[1].Name != "Avocado" {
		t.Fatalf("rename/reorder wrong: %+v", cards)
	}

	// Delete.
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/cards/"+second.ID, nil)
	delRes, _ := http.DefaultClient.Do(req)
	if delRes.StatusCode != 204 {
		t.Fatalf("delete status %d", delRes.StatusCode)
	}
	cards, _ = getList(t, ts.URL)
	if len(cards) != 1 {
		t.Fatalf("after delete want 1 card, got %d", len(cards))
	}
}

// addPhoto posts another image to an existing card.
func addPhoto(t *testing.T, base, cardID string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(makePNG(t, 16, 16))
	mw.Close()

	req, _ := http.NewRequest("POST", base+"/api/cards/"+cardID+"/photos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func getListFull(t *testing.T, base string) (cards []cardDTO, currentID string, currentPhoto int) {
	t.Helper()
	res, err := http.Get(base + "/api/cards")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Cards        []cardDTO `json:"cards"`
		CurrentID    string    `json:"currentId"`
		CurrentPhoto int       `json:"currentPhoto"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Cards, out.CurrentID, out.CurrentPhoto
}

func TestServerPhotosLifecycle(t *testing.T) {
	ts, _ := newTestServer(t)

	// Card starts with one photo.
	res := uploadCard(t, ts.URL, "Apple", true, true)
	var card cardDTO
	json.NewDecoder(res.Body).Decode(&card)
	res.Body.Close()

	// Add a second photo.
	res = addPhoto(t, ts.URL, card.ID)
	if res.StatusCode != 200 {
		t.Fatalf("add photo status %d", res.StatusCode)
	}
	var added photoDTO
	json.NewDecoder(res.Body).Decode(&added)
	res.Body.Close()
	if added.ID == "" {
		t.Fatal("added photo has no id")
	}

	cards, _, photo := getListFull(t, ts.URL)
	if len(cards[0].Photos) != 2 || photo != 0 {
		t.Fatalf("after add: photos=%d currentPhoto=%d", len(cards[0].Photos), photo)
	}

	// Cycle the on-screen photo: 0 -> 1 -> wrap 0, then prev wraps to 1.
	doJSON(t, "POST", ts.URL+"/api/photo/next", "")
	if _, _, p := getListFull(t, ts.URL); p != 1 {
		t.Fatalf("photo/next want 1 got %d", p)
	}
	doJSON(t, "POST", ts.URL+"/api/photo/next", "")
	if _, _, p := getListFull(t, ts.URL); p != 0 {
		t.Fatalf("photo/next wrap want 0 got %d", p)
	}
	doJSON(t, "POST", ts.URL+"/api/photo/prev", "")
	if _, _, p := getListFull(t, ts.URL); p != 1 {
		t.Fatalf("photo/prev wrap want 1 got %d", p)
	}

	// The added photo's image is served.
	imgRes, err := http.Get(ts.URL + "/api/cards/" + card.ID + "/photos/" + added.ID + "/image")
	if err != nil {
		t.Fatal(err)
	}
	if imgRes.StatusCode != 200 {
		t.Fatalf("photo image status %d", imgRes.StatusCode)
	}
	imgRes.Body.Close()

	// Delete the added photo; the index clamps back into range.
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/cards/"+card.ID+"/photos/"+added.ID, nil)
	delRes, _ := http.DefaultClient.Do(req)
	if delRes.StatusCode != 204 {
		t.Fatalf("delete photo status %d", delRes.StatusCode)
	}
	cards, _, photo = getListFull(t, ts.URL)
	if len(cards[0].Photos) != 1 || photo != 0 {
		t.Fatalf("after delete: photos=%d currentPhoto=%d", len(cards[0].Photos), photo)
	}

	// Deleting the only remaining photo is a no-op (use card delete instead).
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/cards/"+card.ID+"/photos/"+cards[0].Photos[0].ID, nil)
	delRes, _ = http.DefaultClient.Do(req)
	if delRes.StatusCode != 204 {
		t.Fatalf("delete last photo status %d", delRes.StatusCode)
	}
	if cards, _, _ := getListFull(t, ts.URL); len(cards[0].Photos) != 1 {
		t.Fatalf("last photo should remain, got %d", len(cards[0].Photos))
	}
}

func TestServerUploadValidation(t *testing.T) {
	ts, _ := newTestServer(t)

	if res := uploadCard(t, ts.URL, "", true, false); res.StatusCode != 400 {
		t.Fatalf("missing name should be 400, got %d", res.StatusCode)
	}
	if res := uploadCard(t, ts.URL, "NoImage", false, true); res.StatusCode != 400 {
		t.Fatalf("missing image should be 400, got %d", res.StatusCode)
	}

	// Non-image bytes under the image field → 400.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "x.png")
	fw.Write([]byte("not a real image"))
	mw.WriteField("name", "Bad")
	mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/cards", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != 400 {
		t.Fatalf("bad image should be 400, got %d", res.StatusCode)
	}
}
