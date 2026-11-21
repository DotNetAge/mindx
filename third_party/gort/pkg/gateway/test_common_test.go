package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type testEnv struct {
	gw      *Server
	ts      *httptest.Server
	addr    string
	cleanup func()
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	mux := http.NewServeMux()
	gw := New()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		gw.handleWS(w, r)
	})

	ts := httptest.NewServer(mux)
	addr := testHostPort(ts)

	go gw.Start()
	time.Sleep(50 * time.Millisecond)

	env := &testEnv{gw: gw, ts: ts, addr: addr}
	env.cleanup = func() {
		gw.Shutdown(context.Background())
		ts.Close()
	}
	t.Cleanup(env.cleanup)
	return env
}

func dialWS(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	u := "ws://" + testHostPort(ts) + "/ws"
	header := http.Header{"Origin": {ts.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(u, header)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	return conn
}

func dialWSWithOrigin(t *testing.T, ts *httptest.Server, origin string) *websocket.Conn {
	t.Helper()
	u := "ws://" + testHostPort(ts) + "/ws"
	header := http.Header{"Origin": {origin}}
	conn, _, err := websocket.DefaultDialer.Dial(u, header)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	return conn
}

func testHostPort(ts *httptest.Server) string {
	_, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
	return "127.0.0.1:" + port
}

func wsSend(conn *websocket.Conn, text string) {
	conn.WriteMessage(websocket.TextMessage, []byte(text))
}

func wsReadText(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	typ, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if typ != websocket.TextMessage {
		t.Fatalf("expected TextMessage, got type %d", typ)
	}
	return string(msg)
}

func wsReadJSON(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()
	raw := wsReadText(t, conn)
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	return m
}

func wsSkipFirst(t *testing.T, conn *websocket.Conn) {
	wsReadText(t, conn)
}

func extractClientID(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	m := wsReadJSON(t, conn)
	if m.ClientID == "" {
		t.Fatal("clientID should not be empty in first message")
	}
	return m.ClientID
}

func wsDoBEGN(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	wsSend(conn, fmt.Sprintf("BEGN|sess-%d|||", time.Now().UnixNano()))
	resp := wsReadText(t, conn)
	parts := strings.Split(resp, "|")
	if len(parts) < 3 || parts[2] != "OK" {
		t.Fatalf("BEGN failed: %s", resp)
	}
	return parts[1]
}

func wsDoTEXT(t *testing.T, conn *websocket.Conn, sessionID, data string) {
	t.Helper()
	wsSend(conn, fmt.Sprintf("TEXT|%s|%s", sessionID, data))
	resp := wsReadText(t, conn)
	if !strings.Contains(resp, "OK") {
		t.Fatalf("TEXT failed: %s", resp)
	}
}

func wsDoCLSE(t *testing.T, conn *websocket.Conn, sessionID string) {
	t.Helper()
	wsSend(conn, fmt.Sprintf("CLSE|%s|||", sessionID))
	resp := wsReadText(t, conn)
	if !strings.Contains(resp, "OK") {
		t.Fatalf("CLSE failed: %s", resp)
	}
}

func setupTestServer(t *testing.T) *testEnv { return setupTestEnv(t) }

func dialTestWS(t *testing.T, ts *httptest.Server) *websocket.Conn { return dialWS(t, ts) }

func extractClientIDFromFirstMessage(t *testing.T, conn *websocket.Conn) string { return extractClientID(t, conn) }

func extractClientIDFromConn(t *testing.T, conn *websocket.Conn) string { return extractClientID(t, conn) }

func doSessionStart(t *testing.T, conn *websocket.Conn) string { return wsDoBEGN(t, conn) }

func doSessionEnd(t *testing.T, conn *websocket.Conn, sessionID string) { wsDoCLSE(t, conn, sessionID) }

func doSendText(t *testing.T, conn *websocket.Conn, sessionID, data string) { wsDoTEXT(t, conn, sessionID, data) }

func setupTestServerW(t *testing.T) *testEnv { return setupTestEnv(t) }

func dialRawWS(t *testing.T, ts *httptest.Server) *websocket.Conn { return dialWS(t, ts) }

func testHostPortW(ts *httptest.Server) string { return testHostPort(ts) }

func setupWSEnv(t *testing.T) *testEnv { return setupTestEnv(t) }

func doBEGN(t *testing.T, conn *websocket.Conn) string { return wsDoBEGN(t, conn) }

func doTEXT(t *testing.T, conn *websocket.Conn, sessionID, data string) { wsDoTEXT(t, conn, sessionID, data) }

func doCLSE(t *testing.T, conn *websocket.Conn, sessionID string) { wsDoCLSE(t, conn, sessionID) }

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write error: %v", err)
	}
}
