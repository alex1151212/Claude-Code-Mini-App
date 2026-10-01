package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
)

func TestLogLevel(t *testing.T) {
	t.Cleanup(func() { logging.SetDebug(false) })
	app := fiber.New()
	app.Get("/logs/level", GetLogLevel)
	app.Put("/logs/level", PutLogLevel)

	call := func(method, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, "/logs/level", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]any{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	if code, out := call("PUT", `{"debug":true}`); code != 200 || out["debug"] != true {
		t.Fatalf("開啟 debug：%d %v", code, out)
	}
	if _, out := call("GET", ""); out["debug"] != true {
		t.Fatalf("GET 應反映已開啟：%v", out)
	}
	if code, out := call("PUT", `{"debug":false}`); code != 200 || out["debug"] != false {
		t.Fatalf("關閉 debug：%d %v", code, out)
	}
	if code, _ := call("PUT", `{}`); code != 400 {
		t.Fatalf("缺 debug 欄位應回 400，got %d", code)
	}
}
