package membrane

import (
	"errors"
	"math"
	"testing"

	"rocalc/internal/casefile"
	"rocalc/internal/validate"
)

// TestEvaluateBrackishDemo 内置苦咸水档端到端结果与手算对照。
func TestEvaluateBrackishDemo(t *testing.T) {
	res, err := Evaluate(casefile.BrackishDemo())
	if err != nil {
		t.Fatalf("内置档核算失败：%v", err)
	}

	// π = 2×0.1×0.08314×298.15 ≈ 4.9576 bar
	if !near(res.Feed.PiBar, 4.9576, 1e-3) {
		t.Errorf("进料渗透压 %.5f，期望 ≈4.9576 bar", res.Feed.PiBar)
	}
	if res.Feed.PiBar < 3 || res.Feed.PiBar > 8 {
		t.Errorf("渗透压 %.3f 不在几个巴量级", res.Feed.PiBar)
	}
	// Δπ = 1.10×4.9576 ≈ 5.4534，NDP = 12−5.4534 ≈ 6.5466
	if !near(res.Operating.OsmoticDeltaBar, 5.4534, 1e-3) {
		t.Errorf("Δπ %.5f，期望 ≈5.4534", res.Operating.OsmoticDeltaBar)
	}
	if !near(res.Operating.NetDrivingPressureBar, 6.5466, 1e-3) {
		t.Errorf("NDP %.5f，期望 ≈6.5466", res.Operating.NetDrivingPressureBar)
	}
	// Qp = 2.5×12×6.5466 ≈ 196.4，Y ≈ 0.1964 ∈ (0,1)
	if res.Membrane.Recovery <= 0 || res.Membrane.Recovery >= 1 {
		t.Fatalf("回收率 %.6f 必须在开区间 (0,1)", res.Membrane.Recovery)
	}
	if !near(res.Membrane.Recovery, 0.1964, 1e-3) {
		t.Errorf("回收率 %.5f，期望 ≈0.1964", res.Membrane.Recovery)
	}
	// r=0.99：Cp=0.001；Cb=0.1·(1−0.001·Y)/(1−Y) ≈ 0.1242
	if !near(res.Salt.ProductMolarity, 0.001, 1e-12) {
		t.Errorf("产水浓度 = %.6f，期望 0.001", res.Salt.ProductMolarity)
	}
	if !near(res.Salt.BrineMolarity, 0.12418, 1e-3) {
		t.Errorf("浓水浓度 %.6f，期望 ≈0.12418", res.Salt.BrineMolarity)
	}
	// 守恒残差必须数值为 0
	if math.Abs(res.Salt.ResidualMolpH) > 1e-8 {
		t.Fatalf("盐守恒残差 %.3e mol/h 必须≈0", res.Salt.ResidualMolpH)
	}
}

// TestEvaluateSaltBalanceInvariant 守恒铁律：进料盐量 = 产水盐量 + 浓水盐量。
func TestEvaluateSaltBalanceInvariant(t *testing.T) {
	for _, r := range []float64{0, 0.5, 0.9, 0.99} {
		c := casefile.BrackishDemo()
		c.Operating.SaltRejection = r
		res, err := Evaluate(c)
		if err != nil {
			t.Fatalf("r=%v 核算失败：%v", r, err)
		}
		lhs := res.Salt.FeedSaltMolpH
		rhs := res.Salt.ProductSaltMolpH + res.Salt.BrineSaltMolpH
		if math.Abs(lhs-rhs) > 1e-8*math.Max(1, math.Abs(lhs)) {
			t.Fatalf("r=%v 盐守恒破坏：%.8f ≠ %.8f", r, lhs, rhs)
		}
	}
}

// TestEvaluateFullRejection r=1：产水含盐为零，盐全部进浓水。
func TestEvaluateFullRejection(t *testing.T) {
	c := casefile.BrackishDemo()
	c.Operating.SaltRejection = 1
	res, err := Evaluate(c)
	if err != nil {
		t.Fatalf("核算失败：%v", err)
	}
	if res.Salt.ProductMolarity != 0 || res.Salt.ProductSaltMolpH != 0 {
		t.Fatalf("r=1 时产水必须不带盐，got Cp=%v salt=%v",
			res.Salt.ProductMolarity, res.Salt.ProductSaltMolpH)
	}
	if math.Abs(res.Salt.BrineSaltMolpH-res.Salt.FeedSaltMolpH) > 1e-8 {
		t.Fatalf("r=1 时盐应全部进浓水：feed=%v brine=%v",
			res.Salt.FeedSaltMolpH, res.Salt.BrineSaltMolpH)
	}
	// Cb = Cf/(1−Y)
	want := res.Salt.FeedMolarity / (1 - res.Membrane.Recovery)
	if !near(res.Salt.BrineMolarity, want, 1e-12) {
		t.Fatalf("r=1 浓水浓度 %.7f 不符合极限式 %.7f", res.Salt.BrineMolarity, want)
	}
}

// TestEvaluatePressureBelowOsmotic 压差低于渗透压：绝不允许报出正通量。
func TestEvaluatePressureBelowOsmotic(t *testing.T) {
	c := casefile.BrackishDemo()
	c.Operating.PolarizationFactor = 1
	c.Operating.AppliedPressureBar = 4.0 // πf≈4.9576，NDP=4−4.9576<0
	res, err := Evaluate(c)
	if err == nil {
		t.Fatalf("Δp<Δπ 必须报错，却返回了 Qp=%.6g Y=%.6g",
			res.Membrane.ProductFlowLpH, res.Membrane.Recovery)
	}
	var de *DomainError
	if !errors.As(err, &de) {
		var ie *validate.InputError
		if !errors.As(err, &ie) {
			t.Fatalf("应返回 DomainError/InputError 带原因，got %T: %v", err, err)
		}
	}
}

// TestEvaluatePressureEqualsOsmoticNoPolarization Δp=πf、β=1：NDP=0，
// 产水流量为零、回收率落不进 (0,1)，必须带原因拒绝。
func TestEvaluatePressureEqualsOsmoticNoPolarization(t *testing.T) {
	c := casefile.BrackishDemo()
	c.Operating.PolarizationFactor = 1
	c.Operating.AppliedPressureBar = 2 * 0.1 * 0.08314 * 298.15
	_, err := Evaluate(c)
	if err == nil {
		t.Fatal("Δp=πf 且无极化时 NDP=0，不应给出有效核算结果")
	}
	if !errors.Is(err, ErrZeroNDP) {
		t.Fatalf("应包裹 ErrZeroNDP，got %v", err)
	}
}

// TestEvaluateRecoveryAtLeastOne Y>=1 非法：这里用超大 Lp 把 Qp 顶到 Qf 以上。
func TestEvaluateRecoveryAtLeastOne(t *testing.T) {
	c := casefile.BrackishDemo()
	c.Membrane.Permeability = 1e6
	_, err := Evaluate(c)
	if err == nil {
		t.Fatal("回收率 ≥1 必须拒绝")
	}
	if !errors.Is(err, ErrRecoveryOutOfRange) {
		t.Fatalf("应包裹 ErrRecoveryOutOfRange，got %v", err)
	}
}

// TestTemperatureOnlyChange 只提温度：π 升、同压差下 NDP 降、Qp 降。
func TestTemperatureOnlyChange(t *testing.T) {
	c1 := casefile.BrackishDemo()
	c2 := c1
	c2.Feed.TemperatureK = 318.15 // 仅升温 20K
	r1, err := Evaluate(c1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Evaluate(c2)
	if err != nil {
		t.Fatal(err)
	}
	if !(r2.Feed.PiBar > r1.Feed.PiBar) {
		t.Fatal("升温后渗透压应升高")
	}
	if !(r2.Operating.NetDrivingPressureBar < r1.Operating.NetDrivingPressureBar) {
		t.Fatal("同压差下升温后 NDP 应下降")
	}
	if !(r2.Membrane.ProductFlowLpH < r1.Membrane.ProductFlowLpH) {
		t.Fatal("同压差下升温后产水流量应下降")
	}
}

// TestEvaluatePureAndIndependent Evaluate 是纯函数：同档两次结果一致，
// 不同档各自独立、互不串名。
func TestEvaluatePureAndIndependent(t *testing.T) {
	a := casefile.BrackishDemo()
	b := casefile.BrackishDemo()
	b.Name = "other"
	b.Feed.TemperatureK = 308.15
	b.Operating.AppliedPressureBar = 20

	r1, err := Evaluate(a)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Evaluate(a)
	if err != nil {
		t.Fatal(err)
	}
	if *r1 != *r2 {
		t.Fatal("同一工况档重复核算结果必须完全一致")
	}
	rb, err := Evaluate(b)
	if err != nil {
		t.Fatal(err)
	}
	if rb.CaseName == r1.CaseName || rb.Feed.PiBar == r1.Feed.PiBar {
		t.Fatal("两份不同工况档的结果发生了串名或串值")
	}
	// 再算一次 a，确认算过 b 之后 a 的结果不受影响
	r3, _ := Evaluate(a)
	if *r1 != *r3 {
		t.Fatal("核算另一份工况档后，原工况档结果被污染")
	}
}

// TestEvaluateMassOnlyPath 只走质量浓度路径（5.844 g/L ÷ 58.44）也应可算。
func TestEvaluateMassOnlyPath(t *testing.T) {
	c := casefile.BrackishDemo()
	c.Feed.Concentration.Molarity = nil
	res, err := Evaluate(c)
	if err != nil {
		t.Fatalf("质量路径核算失败：%v", err)
	}
	if !near(res.Feed.Molarity, 0.1, 1e-9) {
		t.Fatalf("解析摩尔浓度 %.7f，期望 0.1", res.Feed.Molarity)
	}
}
