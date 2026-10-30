package controller

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestIPBlockControllerJSONAndFormRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Fail2ban requires Linux")
	}
	newHostTestDB(t)
	dir := t.TempDir()
	t.Setenv("IPBLOCK_CONTROLLER_STATE", filepath.Join(dir, "state"))
	script := `#!/usr/bin/env python3
import sys,os
p=os.environ['IPBLOCK_CONTROLLER_STATE'];a=sys.argv[1:]
if a==['get','3x-ipl','banip']: print(open(p).read() if os.path.exists(p) else '')
elif a==['get','3x-ipl','bantime']: print('1800')
elif len(a)==4 and a[:2]==['set','3x-ipl'] and a[2] in ['banip','unbanip']:
 with open(p,'w') as f: f.write(a[3] if a[2]=='banip' else '')
 print('1')
else:sys.exit(1)
`
	if err := os.WriteFile(filepath.Join(dir, "fail2ban-client"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	t.Setenv("XUI_LOG_FOLDER", dir)
	a := &ServerController{}
	engine := gin.New()
	engine.POST("/change", a.changeIPBlock)
	for _, tc := range []struct {
		content, body string
		count         int
	}{
		{"application/json", `{"nodeId":0,"ip":"203.0.113.25","action":"ban","email":"client"}`, 1},
		{"application/x-www-form-urlencoded; charset=UTF-8", "nodeId=0&ip=203.0.113.25&action=unban&email=client", 0},
		{"application/x-www-form-urlencoded; charset=UTF-8", "nodeId=0&ip=203.0.113.25&action=ban&email=client", 1},
		{"application/json", `{"nodeId":0,"ip":"203.0.113.25","action":"unban","email":"client"}`, 0},
	} {
		req := httptest.NewRequest("POST", "/change", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.content)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		var env hostEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Success {
			t.Fatalf("%s: %s", tc.content, w.Body.String())
		}
		var result entity.IPBlockStatus
		if err := json.Unmarshal(env.Obj, &result); err != nil {
			t.Fatal(err)
		}
		if !result.Available || len(result.Blocks) != tc.count {
			t.Fatalf("incorrect state: %+v", result)
		}
	}
}
