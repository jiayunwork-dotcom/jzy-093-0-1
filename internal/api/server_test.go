package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rocalc/internal/casefile"
	"rocalc/internal/membrane"
)

func fp(v float64) *float64 { return &v }

func TestHealthAndList(t *testing.T) {
	srv := New(casefile.DefaultRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz 状态码 %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/cases")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list struct {
		Cases []string `json:"cases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Cases) != 1 || list.Cases[0] != "brackish-demo" {
		t.Fatalf("应列出内置档，got %v", list.Cases)
	}
}

// TestNamedCalculate 点名内置档核算，结果与直接内核计算一致。
func TestNamedCalculate(t *testing.T) {
	srv := New(casefile.DefaultRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/cases/brackish-demo/calculate", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("点名核算状态码 %d", resp.StatusCode)
	}
	var res membrane.Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.CaseName != "brackish-demo" {
		t.Fatalf("结果档名 %q", res.CaseName)
	}
	if res.Feed.PiBar < 3 || res.Feed.PiBar > 8 {
		t.Fatalf("渗透压 %.4f 不在几个巴量级", res.Feed.PiBar)
	}
	if math := res.Salt.FeedSaltMolpH - (res.Salt.ProductSaltMolpH + res.Salt.BrineSaltMolpH); math != 0 &&
		(math > 1e-9 || math < -1e-9) {
		t.Fatalf("接口返回结果盐守恒残差 %.3e", math)
	}
}

// TestNamedCalculateNotFound 点错名返回 404。
func TestNamedCalculateNotFound(t *testing.T) {
	srv := New(casefile.NewRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/cases/nope/calculate", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("缺失档应 404，got %d", resp.StatusCode)
	}
}

// TestAdhocCalculate 临时拼一份档直接算。
func TestAdhocCalculate(t *testing.T) {
	srv := New(casefile.NewRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
	  "name":"adhoc",
	  "feed":{"concentration":{"solute":"NaCl","molarity_mol_L":0.1,"van_t_hoff_factor":2},
	          "temperature_K":298.15,"feed_flow_L_p_h":1000},
	  "membrane":{"area_m2":2.5,"hydraulic_permeability_L_p_h_p_m2_p_bar":12},
	  "operating":{"applied_pressure_bar":12,"polarization_factor":1.1,"salt_rejection":0.99}
	}`
	resp, err := http.Post(ts.URL+"/calculate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		t.Fatalf("临时核算应 200，got %d body=%s", resp.StatusCode, buf.String())
	}
	var res membrane.Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.Membrane.Recovery <= 0 || res.Membrane.Recovery >= 1 {
		t.Fatalf("回收率 %.6f 不在 (0,1)", res.Membrane.Recovery)
	}
}

// TestAdhocPressureBelowOsmotic 接口级铁律：压差低于渗透压，
// 必须 422 且响应里没有正通量。
func TestAdhocPressureBelowOsmotic(t *testing.T) {
	srv := New(casefile.NewRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
	  "name":"lowp",
	  "feed":{"concentration":{"molarity_mol_L":0.1,"van_t_hoff_factor":2},
	          "temperature_K":298.15,"feed_flow_L_p_h":1000},
	  "membrane":{"area_m2":2.5,"hydraulic_permeability_L_p_h_p_m2_p_bar":12},
	  "operating":{"applied_pressure_bar":4,"polarization_factor":1,"salt_rejection":0.99}
	}`
	resp, err := http.Post(ts.URL+"/calculate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("Δp<Δπ 应返回 422，got %d", resp.StatusCode)
	}
	var eb struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatal(err)
	}
	if eb.Code != "invalid_operating_point" || !strings.Contains(eb.Message, "NDP") {
		t.Fatalf("错误响应应带 NDP 原因，got %+v", eb)
	}
}

// TestAdhocInvalidInput 负温度等输入问题返回 422 并罗列原因。
func TestAdhocInvalidInput(t *testing.T) {
	srv := New(casefile.NewRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
	  "name":"bad",
	  "feed":{"concentration":{"molarity_mol_L":-0.1,"van_t_hoff_factor":2},
	          "temperature_K":0,"feed_flow_L_p_h":1000},
	  "membrane":{"area_m2":0,"hydraulic_permeability_L_p_h_p_m2_p_bar":12},
	  "operating":{"applied_pressure_bar":12,"polarization_factor":0.5,"salt_rejection":2}
	}`
	resp, err := http.Post(ts.URL+"/calculate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("非法输入应 422，got %d", resp.StatusCode)
	}
	var eb struct {
		Code    string   `json:"code"`
		Message []string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatal(err)
	}
	if len(eb.Message) < 4 {
		t.Fatalf("应罗列全部非法原因，got %v", eb.Message)
	}
}

// TestRegisterDuplicateAndUnknownField 登记与 JSON 严格性。
func TestRegisterDuplicateAndUnknownField(t *testing.T) {
	srv := New(casefile.NewRegistry())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	raw, _ := json.Marshal(casefile.BrackishDemo())
	resp, err := http.Post(ts.URL+"/cases", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("首次登记应 201，got %d", resp.StatusCode)
	}
	resp, err = http.Post(ts.URL+"/cases", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重复登记应 409，got %d", resp.StatusCode)
	}

	resp, err = http.Post(ts.URL+"/calculate", "application/json",
		strings.NewReader(`{"name":"x","bogus":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知 JSON 字段应 400，got %d", resp.StatusCode)
	}
}

// TestTwoCasesSameProcess 同进程两份档经 HTTP 反复核算，各自独立不串值。
func TestTwoCasesSameProcess(t *testing.T) {
	reg := casefile.NewRegistry()
	a := casefile.BrackishDemo()
	b := casefile.BrackishDemo()
	b.Name = "hot-highp"
	b.Feed.TemperatureK = 318.15
	b.Operating.AppliedPressureBar = 20
	if err := reg.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(b); err != nil {
		t.Fatal(err)
	}

	srv := New(reg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	calc := func(name string) membrane.Result {
		t.Helper()
		resp, err := http.Post(ts.URL+"/cases/"+name+"/calculate", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s 核算状态码 %d", name, resp.StatusCode)
		}
		var res membrane.Result
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	r1 := calc("brackish-demo")
	r2 := calc("hot-highp")
	r3 := calc("brackish-demo")
	if r1.CaseName == r2.CaseName || r1.Feed.TemperatureK == r2.Feed.TemperatureK {
		t.Fatal("两份档结果串名或串值")
	}
	if r1.Feed.PiBar != r3.Feed.PiBar || r1.Membrane.ProductFlowLpH != r3.Membrane.ProductFlowLpH {
		t.Fatal("同一份档两次核算结果不一致，疑似共享状态被污染")
	}
	if r2.Feed.PiBar <= r1.Feed.PiBar {
		t.Fatal("升温档渗透压应更高")
	}
}
