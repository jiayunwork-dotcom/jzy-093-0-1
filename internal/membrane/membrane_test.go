package membrane

import (
	"errors"
	"math"
	"testing"
)

func fp(v float64) *float64 { return &v }

func near(got, want, rel float64) bool {
	return math.Abs(got-want) <= rel*math.Max(math.Abs(want), 1e-12)
}

func TestNetDrivingPressure(t *testing.T) {
	// NDP = Δp − Δπ
	if got := NetDrivingPressure(12, 5.45); !near(got, 6.55, 1e-12) {
		t.Fatalf("NDP = %.4f，期望 6.55", got)
	}
}

// TestFluxZeroAtNDPZero 无极化、Δp 恰好等于进料渗透压时产水流量为 0。
func TestFluxZeroAtNDPZero(t *testing.T) {
	q, err := Flux(2.5, 12, 0)
	if err != nil {
		t.Fatalf("NDP=0 不应作为原语错误，got %v", err)
	}
	if q != 0 {
		t.Fatalf("NDP=0 时产水流量必须为 0，got %.6g", q)
	}
}

// TestFluxRejectsNegativeNDP 最关键防线：NDP 为负绝不能冒出正通量。
func TestFluxRejectsNegativeNDP(t *testing.T) {
	q, err := Flux(2.5, 12, -1.0)
	if err == nil {
		t.Fatalf("NDP 为负必须报错，却返回了通量 %.6g", q)
	}
	if q != 0 {
		t.Fatalf("NDP 为负时返回的流量必须是 0，got %.6g", q)
	}
	var de *DomainError
	if !errors.As(err, &de) || !errors.Is(err, ErrNegativeNDP) {
		t.Fatalf("应返回包裹 ErrNegativeNDP 的 DomainError，got %v", err)
	}
}

func TestFluxFormula(t *testing.T) {
	// Qp = A·Lp·NDP = 2.5 × 12 × 6.54664
	got, err := Flux(2.5, 12, 6.54664)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	want := 2.5 * 12 * 6.54664
	if !near(got, want, 1e-12) {
		t.Fatalf("Qp = %.6f，期望 %.6f", got, want)
	}
}

// TestBrineConcentrationFullRejection r=1 时走极限 Cb=Cf/(1−Y)，Cp=0。
func TestBrineConcentrationFullRejection(t *testing.T) {
	cb, err := BrineConcentration(0.1, 0.2, 1)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if !near(cb, 0.125, 1e-12) {
		t.Fatalf("r=1 时 Cb = %.7f，期望 0.125", cb)
	}
}

// TestBrineConcentrationPartialRejection r<1 的物料守恒通式。
func TestBrineConcentrationPartialRejection(t *testing.T) {
	// Cb = Cf·(1−(1−r)Y)/(1−Y)
	cb, err := BrineConcentration(0.1, 0.2, 0.99)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	want := 0.1 * (1 - 0.01*0.2) / 0.8
	if !near(cb, want, 1e-12) {
		t.Fatalf("Cb = %.8f，期望 %.8f", cb, want)
	}

	// 物料守恒：Cf·Qf = Cp·Qp + Cb·Qb
	cp := (1 - 0.99) * 0.1
	qf, qp := 1000.0, 200.0
	qb := qf - qp
	res := 0.1*qf - (cp*qp + cb*qb)
	if math.Abs(res) > 1e-9 {
		t.Fatalf("盐守恒残差 %.3e 必须为 0", res)
	}
}

// TestBrineConcentrationRaisesWithRecovery 只提高回收率，浓水浓度必须上升。
func TestBrineConcentrationRaisesWithRecovery(t *testing.T) {
	c1, _ := BrineConcentration(0.1, 0.2, 0.99)
	c2, _ := BrineConcentration(0.1, 0.5, 0.99)
	c3, _ := BrineConcentration(0.1, 0.8, 0.99)
	if !(c2 > c1 && c3 > c2) {
		t.Fatalf("回收率升高浓水应变浓：%.5f %.5f %.5f", c1, c2, c3)
	}
}

// TestBrineConcentrationNeverUsesDilutionForm 守护最致命的坑：
// 任何回收率下浓水都不能比进料还稀（r<1 时也仅因透盐略微下降）。
func TestBrineConcentrationNeverUsesDilutionForm(t *testing.T) {
	for _, y := range []float64{0.1, 0.3, 0.6, 0.9} {
		cb, err := BrineConcentration(0.1, y, 1)
		if err != nil {
			t.Fatalf("意外错误：%v", err)
		}
		if cb <= 0.1 {
			t.Fatalf("Y=%.1f、r=1 时 Cb=%.6f 必须大于 Cf=0.1（杜绝 Cf·(1−Y) 写法）", y, cb)
		}
	}
}

func TestBrineConcentrationRejectsBadRecovery(t *testing.T) {
	for _, y := range []float64{0, -0.1, 1, 1.2} {
		if _, err := BrineConcentration(0.1, y, 0.99); !errors.Is(err, ErrRecoveryOutOfRange) {
			t.Fatalf("Y=%v 应返回 ErrRecoveryOutOfRange，got %v", y, err)
		}
	}
}
