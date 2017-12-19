/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tabletenv

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/youtube/vitess/go/sqltypes"
	"github.com/youtube/vitess/go/vt/callinfo"
	"github.com/youtube/vitess/go/vt/callinfo/fakecallinfo"
	querypb "github.com/youtube/vitess/go/vt/proto/query"
	"github.com/youtube/vitess/go/vt/servenv"
)

func TestLogStats(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	logStats.AddRewrittenSQL("sql1", time.Now())

	if !strings.Contains(logStats.RewrittenSQL(), "sql1") {
		t.Fatalf("RewrittenSQL should contains sql: sql1")
	}

	if logStats.SizeOfResponse() != 0 {
		t.Fatalf("there is no rows in log stats, estimated size should be 0 bytes")
	}

	logStats.Rows = [][]sqltypes.Value{{sqltypes.NewVarBinary("a")}}
	if logStats.SizeOfResponse() <= 0 {
		t.Fatalf("log stats has some rows, should have positive response size")
	}

	params := map[string][]string{"full": {}}

	logStats.Format(url.Values(params))
}

func TestLogStatsFormat(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	logStats.StartTime = time.Date(2017, time.January, 1, 1, 2, 3, 0, time.UTC)
	logStats.EndTime = time.Date(2017, time.January, 1, 1, 2, 4, 0, time.UTC)
	logStats.OriginalSQL = "sql1"
	logStats.AddRewrittenSQL("sql1", time.Now())

	logStats.Rows = [][]sqltypes.Value{{sqltypes.NewVarBinary("a")}}

	params := map[string][]string{"full": {}}
	got := logStats.Format(url.Values(params))
	want := "test\t\t\t''\t''\tJan  1 01:02:03.000000\tJan  1 01:02:04.000000\t1.000000\t\t\"sql1\"\tmap[]\t1\t\"sql1\"\tmysql\t0.000000\t0.000000\t0\t1\t\"\"\t\n"
	if got != want {
		t.Errorf("logstats text format: got:\n%q\nwant:\n%q\n", got, want)
	}

	*servenv.QueryLogFormat = "json"
	got = logStats.Format(url.Values(params))
	want = "{\"Method\": \"test\", \"RemoteAddr\": \"\", \"Username\": \"\", \"ImmediateCaller\": \"\", \"Effective Caller\": \"\", \"Start\": \"Jan  1 01:02:03.000000\", \"End\": \"Jan  1 01:02:04.000000\", \"TotalTime\": 1.000000, \"PlanType\": \"\", \"OriginalSQL\": \"sql1\", \"BindVars\": \"map[]\", \"Queries\": 1, \"RewrittenSQL\": \"sql1\", \"QuerySources\": \"mysql\", \"MysqlTime\": 0.000000, \"ConnWaitTime\": 0.000000, \"RowsAffected\": 0, \"ResponseSize\": 1, \"Error\": \"\"}\n"
	if got != want {
		t.Errorf("logstats text format: got:\n%v\nwant:\n%v\n", got, want)
	}

	*servenv.QueryLogFormat = "text"
}

func TestLogStatsFormatBindVariables(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	logStats.BindVariables = map[string]*querypb.BindVariable{
		"key_1": sqltypes.StringBindVariable("val_1"),
		"key_2": sqltypes.Int64BindVariable(789),
	}

	formattedStr := logStats.FmtBindVariables(true)
	if !strings.Contains(formattedStr, "key_1") ||
		!strings.Contains(formattedStr, "val_1") {
		t.Fatalf("bind variable 'key_1': 'val_1' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_2") ||
		!strings.Contains(formattedStr, "789") {
		t.Fatalf("bind variable 'key_2': '789' is not formatted")
	}

	logStats.BindVariables["key_3"] = sqltypes.BytesBindVariable([]byte("val_3"))
	formattedStr = logStats.FmtBindVariables(false)
	if !strings.Contains(formattedStr, "key_1") {
		t.Fatalf("bind variable 'key_1' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_2") ||
		!strings.Contains(formattedStr, "789") {
		t.Fatalf("bind variable 'key_2': '789' is not formatted")
	}
	if !strings.Contains(formattedStr, "key_3") {
		t.Fatalf("bind variable 'key_3' is not formatted")
	}
}

func TestLogStatsFormatQuerySources(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	if logStats.FmtQuerySources() != "none" {
		t.Fatalf("should return none since log stats does not have any query source, but got: %s", logStats.FmtQuerySources())
	}

	logStats.QuerySources |= QuerySourceMySQL
	if !strings.Contains(logStats.FmtQuerySources(), "mysql") {
		t.Fatalf("'mysql' should be in formated query sources")
	}

	logStats.QuerySources |= QuerySourceConsolidator
	if !strings.Contains(logStats.FmtQuerySources(), "consolidator") {
		t.Fatalf("'consolidator' should be in formated query sources")
	}
}

func TestLogStatsContextHTML(t *testing.T) {
	html := "HtmlContext"
	callInfo := &fakecallinfo.FakeCallInfo{
		Html: html,
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats := NewLogStats(ctx, "test")
	if string(logStats.ContextHTML()) != html {
		t.Fatalf("expect to get html: %s, but got: %s", html, string(logStats.ContextHTML()))
	}
}

func TestLogStatsErrorStr(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	if logStats.ErrorStr() != "" {
		t.Fatalf("should not get error in stats, but got: %s", logStats.ErrorStr())
	}
	errStr := "unknown error"
	logStats.Error = errors.New(errStr)
	if !strings.Contains(logStats.ErrorStr(), errStr) {
		t.Fatalf("expect string '%s' in error message, but got: %s", errStr, logStats.ErrorStr())
	}
}

func TestLogStatsRemoteAddrUsername(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test")
	addr, user := logStats.RemoteAddrUsername()
	if addr != "" {
		t.Fatalf("remote addr should be empty")
	}
	if user != "" {
		t.Fatalf("username should be empty")
	}

	remoteAddr := "1.2.3.4"
	username := "vt"
	callInfo := &fakecallinfo.FakeCallInfo{
		Remote: remoteAddr,
		User:   username,
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats = NewLogStats(ctx, "test")
	addr, user = logStats.RemoteAddrUsername()
	if addr != remoteAddr {
		t.Fatalf("expected to get remote addr: %s, but got: %s", remoteAddr, addr)
	}
	if user != username {
		t.Fatalf("expected to get username: %s, but got: %s", username, user)
	}
}
