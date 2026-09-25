package osmotic

import (
	"errors"
	"math"
	"testing"
)

func approx(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol
}

// TestBrackishOrder 内置苦咸水档进料（0.1 mol/L NaCl，298.15K）
// 的渗透压应落在“几个巴”的量级：π = 2×0.1×0.08314×298.15 ≈ 4.96 bar。
func TestBrackishOrder(t *testing.T) {
	pi, err := Pressure(0.10, 2.0, 298.15)
	if err != nil {
		t.Fatalf("计算失败：%v", err)
	}
	if !approx(pi, 4.9576, 1e-3) {
		t.Fatalf("渗透压 = %.5f bar，期望约 4.9576 bar", pi)
	}
	if pi < 3 || pi > 8 {
		t.Fatalf("渗透压 %.3f bar 不在苦咸水“几个巴”量级", pi)
	}
}

// TestVanTHoffFormula 直接核对 van't Hoff 关系式。
func TestVanTHoffFormula(t *testing.T) {
	cases := []struct {
		c, i, temp, want float64
	}{
		{0.10, 2.0, 298.15, 2 * 0.10 * RBarL * 298.15},
		{0.25, 2.0, 303.15, 2 * 0.25 * RBarL * 303.15},
		{0.05, 1.0, 320.0, 1 * 0.05 * RBarL * 320.0},
	}
	for _, tc := range cases {
		got, err := Pressure(tc.c, tc.i, tc.temp)
		if err != nil {
			t.Fatalf("Pressure(%v,%v,%v) 意外错误：%v", tc.c, tc.i, tc.temp, err)
		}
		if !approx(got, tc.want, 1e-12) {
			t.Errorf("Pressure(%v,%v,%v) = %.6f，期望 %.6f", tc.c, tc.i, tc.temp, got, tc.want)
		}
	}
}

// TestTemperatureRaisesPi 交叉关系：只提温度，渗透压升高。
func TestTemperatureRaisesPi(t *testing.T) {
	low, _ := Pressure(0.10, 2.0, 288.15)
	high, _ := Pressure(0.10, 2.0, 308.15)
	if !(high > low) {
		t.Fatalf("升温后渗透压应升高：%.5f !> %.5f", high, low)
	}
}

// TestZeroConcentration 允许零浓度，渗透压为 0。
func TestZeroConcentration(t *testing.T) {
	pi, err := Pressure(0, 2.0, 298.15)
	if err != nil || pi != 0 {
		t.Fatalf("零浓度应返回零渗透压，got pi=%v err=%v", pi, err)
	}
}

func TestRejectInvalid(t *testing.T) {
	cases := []struct {
		name       string
		c, i, temp float64
		want       error
	}{
		{"负浓度", -0.01, 2.0, 298.15, ErrNegativeMolarity},
		{"温度为零", 0.10, 2.0, 0, ErrNonPositiveTemperature},
		{"温度为负", 0.10, 2.0, -10, ErrNonPositiveTemperature},
		{"因子为零", 0.10, 0, 298.15, ErrNonPositiveFactor},
		{"因子为负", 0.10, -2, 298.15, ErrNonPositiveFactor},
		{"NaN 温度", 0.10, 2.0, math.NaN(), ErrNonFinite},
		{"Inf 浓度", math.Inf(1), 2.0, 298.15, ErrNonFinite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Pressure(tc.c, tc.i, tc.temp)
			if !errors.Is(err, tc.want) {
				t.Fatalf("期望错误 %v，实际 %v", tc.want, err)
			}
		})
	}
}
