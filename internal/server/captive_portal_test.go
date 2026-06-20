package server

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"hostberry/internal/constants"
)

func captivePortalRequestCtx(clientIP, path string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI(path)
	addr, err := net.ResolveTCPAddr("tcp", clientIP+":12345")
	if err != nil {
		panic(err)
	}
	ctx.SetRemoteAddr(addr)
	return ctx
}

func TestCaptivePortalDetectHandlerFromAP(t *testing.T) {
	app := fiber.New()
	app.Get("/generate_204", CaptivePortalDetectHandler)

	ctx := captivePortalRequestCtx("192.168.4.50", "/generate_204")
	app.Handler()(ctx)

	if got := ctx.Response.StatusCode(); got != fiber.StatusFound {
		t.Fatalf("status = %d, want %d", got, fiber.StatusFound)
	}
	loc := string(ctx.Response.Header.Peek("Location"))
	if loc != constants.DefaultAPSetupURL {
		t.Fatalf("Location = %q, want %q", loc, constants.DefaultAPSetupURL)
	}
}

func TestCaptivePortalDetectHandlerFromLAN(t *testing.T) {
	app := fiber.New()
	app.Get("/generate_204", CaptivePortalDetectHandler)

	ctx := captivePortalRequestCtx("192.168.1.50", "/generate_204")
	app.Handler()(ctx)

	if got := ctx.Response.StatusCode(); got != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", got, fiber.StatusNoContent)
	}
}

func TestCaptivePortalLandingHandlerFromAP(t *testing.T) {
	app := fiber.New()
	app.Get("/portal", CaptivePortalLandingHandler)

	ctx := captivePortalRequestCtx("192.168.4.50", "/portal")
	app.Handler()(ctx)

	if got := ctx.Response.StatusCode(); got != fiber.StatusFound {
		t.Fatalf("status = %d, want %d", got, fiber.StatusFound)
	}
	loc := string(ctx.Response.Header.Peek("Location"))
	if loc != constants.DefaultAPSetupURL {
		t.Fatalf("Location = %q", loc)
	}
}

func TestCaptivePortalAPIHandlerFromAP(t *testing.T) {
	app := fiber.New()
	app.Get("/api/captive-portal", CaptivePortalAPIHandler)

	ctx := captivePortalRequestCtx("192.168.4.50", "/api/captive-portal")
	app.Handler()(ctx)

	if got := ctx.Response.StatusCode(); got != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", got, fiber.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["captive"] != true {
		t.Fatalf("captive = %v, want true", payload["captive"])
	}
	if payload["user-portal-url"] != constants.DefaultAPSetupURL {
		t.Fatalf("user-portal-url = %v", payload["user-portal-url"])
	}
}

func TestCaptivePortalAPIHandlerFromLAN(t *testing.T) {
	app := fiber.New()
	app.Get("/api/captive-portal", CaptivePortalAPIHandler)

	ctx := captivePortalRequestCtx("192.168.1.50", "/api/captive-portal")
	app.Handler()(ctx)

	var payload map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["captive"] != false {
		t.Fatalf("captive = %v, want false", payload["captive"])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
